package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mixigroup/mixi2-application-sample-go/config"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const stateFile = "state.json"
const channelID = "UCtEbxUxhVwEwFuNzTjrpzOg"
const releasesPageURL = "https://www.youtube.com/@FUSE_AKIRA_/releases"

var releasesPageVideoIDRe = regexp.MustCompile(
	`(?:watch\?v=|/watch\?v=|"videoId":"|\\\"videoId\\\":\\\")([a-zA-Z0-9_-]{11})`,
)

var httpClient = &http.Client{
	Timeout: 20 * time.Second,
}

type State struct {
	PostedVideoIDs []string `json:"posted_video_ids"`
	LastSource     string   `json:"last_source"`
	LastGroupID    string   `json:"last_group_id"`
}

type YouTubeVideo struct {
	ID         string
	Title      string
	URL        string
	Source     string
	GroupID    string
	GroupTitle string
}

type VideoBuckets struct {
	Videos        []YouTubeVideo
	ReleasePage   []YouTubeVideo
	ReleaseSearch []YouTubeVideo
	Playlists     []YouTubeVideo
}

type ChannelsResponse struct {
	Items []struct {
		ContentDetails struct {
			RelatedPlaylists struct {
				Uploads string `json:"uploads"`
			} `json:"relatedPlaylists"`
		} `json:"contentDetails"`
	} `json:"items"`
}

type PlaylistItemsResponse struct {
	NextPageToken string `json:"nextPageToken"`
	Items         []struct {
		Snippet struct {
			Title      string `json:"title"`
			ResourceID struct {
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
		} `json:"snippet"`
	} `json:"items"`
}

type SearchResponse struct {
	NextPageToken string `json:"nextPageToken"`
	Items         []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
}

type VideosResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
}

type PlaylistsResponse struct {
	NextPageToken string `json:"nextPageToken"`
	Items         []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
}

func loadState() State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return State{}
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}
	}
	return s
}

func saveState(s State) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Fatal("state.json変換失敗:", err)
	}
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		log.Fatal("state.json保存失敗:", err)
	}
}

func buildPostedMap(state State) map[string]bool {
	posted := make(map[string]bool, len(state.PostedVideoIDs))

	for _, id := range state.PostedVideoIDs {
		posted[id] = true
	}

	return posted
}

func getJSON(requestURL string, v any) error {
	resp, err := httpClient.Get(requestURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API取得失敗: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

func isUnavailableTitle(title string) bool {
	t := strings.TrimSpace(strings.ToLower(title))
	return t == "" ||
		t == "private video" ||
		t == "[private video]" ||
		t == "deleted video" ||
		t == "[deleted video]"
}

func addVideo(videos map[string]YouTubeVideo, videoID, title, source, groupID, groupTitle string) {
	if videoID == "" || isUnavailableTitle(title) {
		return
	}

	if _, exists := videos[videoID]; exists {
		return
	}

	videos[videoID] = YouTubeVideo{
		ID:         videoID,
		Title:      title,
		URL:        "https://youtu.be/" + videoID,
		Source:     source,
		GroupID:    groupID,
		GroupTitle: groupTitle,
	}
}

func mapToSlice(videos map[string]YouTubeVideo) []YouTubeVideo {
	result := make([]YouTubeVideo, 0, len(videos))
	for _, video := range videos {
		result = append(result, video)
	}
	return result
}

func getUploadsPlaylistID(apiKey string) (string, error) {
	requestURL := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/channels?part=contentDetails&id=%s&key=%s",
		url.QueryEscape(channelID),
		url.QueryEscape(apiKey),
	)

	var result ChannelsResponse
	if err := getJSON(requestURL, &result); err != nil {
		return "", err
	}

	if len(result.Items) == 0 {
		return "", fmt.Errorf("チャンネルが見つかりませんでした")
	}

	return result.Items[0].ContentDetails.RelatedPlaylists.Uploads, nil
}

func fetchVideosFromPlaylist(apiKey, playlistID, source, groupTitle string) ([]YouTubeVideo, error) {
	pageToken := ""
	videos := make(map[string]YouTubeVideo)

	for {
		requestURL := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&playlistId=%s&maxResults=50&key=%s&pageToken=%s",
			url.QueryEscape(playlistID),
			url.QueryEscape(apiKey),
			url.QueryEscape(pageToken),
		)

		var result PlaylistItemsResponse
		if err := getJSON(requestURL, &result); err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			addVideo(videos, item.Snippet.ResourceID.VideoID, item.Snippet.Title, source, playlistID, groupTitle)
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return mapToSlice(videos), nil
}

func fetchVideosFromSearch(apiKey string) ([]YouTubeVideo, error) {
	requestURL := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/search?part=snippet&channelId=%s&type=video&order=date&maxResults=50&key=%s",
		url.QueryEscape(channelID),
		url.QueryEscape(apiKey),
	)

	var result SearchResponse
	if err := getJSON(requestURL, &result); err != nil {
		return nil, err
	}

	videos := make(map[string]YouTubeVideo)

	for _, item := range result.Items {
		addVideo(videos, item.ID.VideoID, item.Snippet.Title, "release-api", "release-api", "API検索")
	}

	return mapToSlice(videos), nil
}

func fetchVideosFromReleasesPage(apiKey string) ([]YouTubeVideo, error) {
	req, err := http.NewRequest("GET", releasesPageURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "ja,en-US;q=0.9,en;q=0.8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("releasesページ取得失敗: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}

	body := string(bodyBytes)

	matches := releasesPageVideoIDRe.FindAllStringSubmatch(body, -1)

	seen := make(map[string]bool) // releasesページ内の重複動画IDを除外する
	var ids []string

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}

		id := m[1]
		if seen[id] {
			continue
		}

		seen[id] = true
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("releasesページから動画IDを取得できませんでした")
	}

	videos := make(map[string]YouTubeVideo)

	for i := 0; i < len(ids); i += 50 {
		end := i + 50
		if end > len(ids) {
			end = len(ids)
		}

		requestURL := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/videos?part=snippet&id=%s&key=%s",
			url.QueryEscape(strings.Join(ids[i:end], ",")),
			url.QueryEscape(apiKey),
		)

		var result VideosResponse
		if err := getJSON(requestURL, &result); err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			addVideo(videos, item.ID, item.Snippet.Title, "release-page", "release-page", "リリースページ")
		}
	}

	return mapToSlice(videos), nil
}

func fetchVideosFromAllPlaylists(apiKey string) ([]YouTubeVideo, error) {
	pageToken := ""
	allVideos := make(map[string]YouTubeVideo)

	for {
		requestURL := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/playlists?part=snippet&channelId=%s&maxResults=50&key=%s&pageToken=%s",
			url.QueryEscape(channelID),
			url.QueryEscape(apiKey),
			url.QueryEscape(pageToken),
		)

		var result PlaylistsResponse
		if err := getJSON(requestURL, &result); err != nil {
			return nil, err
		}

		for _, playlist := range result.Items {
			if playlist.ID == "" {
				continue
			}

			videos, err := fetchVideosFromPlaylist(apiKey, playlist.ID, "playlist", playlist.Snippet.Title)
			if err != nil {
				log.Println("再生リスト取得スキップ:", playlist.Snippet.Title, err)
				continue
			}

			for _, video := range videos {
				addVideo(allVideos, video.ID, video.Title, video.Source, video.GroupID, video.GroupTitle)
			}
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return mapToSlice(allVideos), nil
}

func fetchAllVideos(apiKey string) (VideoBuckets, error) {
	uploadsPlaylistID, err := getUploadsPlaylistID(apiKey)
	if err != nil {
		return VideoBuckets{}, err
	}

	videos, err := fetchVideosFromPlaylist(apiKey, uploadsPlaylistID, "video", "動画")
	if err != nil {
		return VideoBuckets{}, err
	}

	releasePage, err := fetchVideosFromReleasesPage(apiKey)
	if err != nil {
		log.Println("releasesページ取得失敗。今回はリリースページ枠を空にします:", err)
		releasePage = []YouTubeVideo{}
	}

	releaseSearch, err := fetchVideosFromSearch(apiKey)
	if err != nil {
		log.Println("API検索取得失敗。今回はAPI検索枠を空にします:", err)
		releaseSearch = []YouTubeVideo{}
	}

	playlists, err := fetchVideosFromAllPlaylists(apiKey)
	if err != nil {
		return VideoBuckets{}, err
	}

	return VideoBuckets{
		Videos:        videos,
		ReleasePage:   releasePage,
		ReleaseSearch: releaseSearch,
		Playlists:     playlists,
	}, nil
}

func sourceFamily(source string) string {
	switch source {
	case "release", "release-page", "release-api":
		return "release"
	default:
		return source
	}
}

func filterCandidates(videos []YouTubeVideo, state State, posted map[string]bool) []YouTubeVideo {
	var candidates []YouTubeVideo

	lastFamily := sourceFamily(state.LastSource)

	for _, video := range videos {
		if posted[video.ID] {
			continue
		}

		videoFamily := sourceFamily(video.Source)

		if lastFamily == "release" && videoFamily == "release" {
			continue
		}

		if state.LastSource == "playlist" &&
			video.Source == "playlist" &&
			state.LastGroupID == video.GroupID {
			continue
		}

		candidates = append(candidates, video)
	}

	return candidates
}

func chooseWeighted(candidates VideoBuckets) (YouTubeVideo, bool) {
	type bucket struct {
		Weight int
		Videos []YouTubeVideo
	}

	buckets := []bucket{
		{Weight: 30, Videos: candidates.Videos},
		{Weight: 45, Videos: candidates.ReleasePage},
		{Weight: 15, Videos: candidates.ReleaseSearch},
		{Weight: 10, Videos: candidates.Playlists},
	}

	totalWeight := 0
	for _, b := range buckets {
		if len(b.Videos) > 0 {
			totalWeight += b.Weight
		}
	}

	if totalWeight == 0 {
		return YouTubeVideo{}, false
	}

	r := rand.IntN(totalWeight)
	current := 0

	for _, b := range buckets {
		if len(b.Videos) == 0 {
			continue
		}

		current += b.Weight
		if r < current {
			return b.Videos[rand.IntN(len(b.Videos))], true
		}
	}

	return YouTubeVideo{}, false
}

func main() {
	cfg := config.GetConfig()

	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		log.Fatal("YOUTUBE_API_KEY missing value")
	}

	buckets, err := fetchAllVideos(apiKey)
	if err != nil {
		log.Fatal("YouTube動画取得失敗:", err)
	}

	state := loadState()
	posted := buildPostedMap(state)

	candidates := VideoBuckets{
		Videos:        filterCandidates(buckets.Videos, state, posted),
		ReleasePage:   filterCandidates(buckets.ReleasePage, state, posted),
		ReleaseSearch: filterCandidates(buckets.ReleaseSearch, state, posted),
		Playlists:     filterCandidates(buckets.Playlists, state, posted),
	}

	target, ok := chooseWeighted(candidates)
	if !ok {
		log.Println("未投稿の公式動画がありません")
		return
	}

	authenticator, err := auth.NewAuthenticator(
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.TokenURL,
	)
	if err != nil {
		log.Fatal("認証設定作成失敗:", err)
	}

	ctx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		log.Fatal("認証失敗:", err)
	}

	conn, err := grpc.NewClient(
		cfg.APIAddress,
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		),
	)
	if err != nil {
		log.Fatal("mixi2 API接続失敗:", err)
	}
	defer conn.Close()

	client := application_apiv1.NewApplicationServiceClient(conn)

	title := trimTitle(target.Title, target.URL)
	text := "今日の布施明ヽ('∀')ﾉ\n\n" + title + "\n\n" + target.URL

	if os.Getenv("PREVIEW") == "1" {
		log.Println("プレビュー:")
		log.Println(text)
		log.Println("source:", target.Source, "group:", target.GroupTitle)
		return
	}

	_, err = client.CreatePost(
		ctx,
		&application_apiv1.CreatePostRequest{
			Text: text,
		},
	)
	if err != nil {
		log.Fatal("投稿失敗:", err)
	}

	log.Println("投稿成功:", target.Title, "source:", target.Source, "group:", target.GroupTitle)

	state.PostedVideoIDs = append(state.PostedVideoIDs, target.ID)
	state.LastSource = target.Source
	state.LastGroupID = target.GroupID
	saveState(state)
}

func trimTitle(title string, videoURL string) string {
	// mixi2投稿本文の上限に収まるようにするための最大文字数
	const maxLen = 147

	prefix := "今日の布施明ヽ('∀')ﾉ\n\n"
	suffix := "\n\n" + videoURL

	available := maxLen - len([]rune(prefix)) - len([]rune(suffix))
	if available <= 0 {
		return ""
	}

	runes := []rune(title)
	if len(runes) <= available {
		return title
	}

	return string(runes[:available-1]) + "…"
}
