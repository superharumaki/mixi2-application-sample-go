package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/mixigroup/mixi2-application-sample-go/config"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const stateFile = "state.json"
const channelID = "UCtEbxUxhVwEwFuNzTjrpzOg"

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
	Videos    []YouTubeVideo
	Releases  []YouTubeVideo
	Playlists []YouTubeVideo
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
		log.Fatal(err)
	}
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		log.Fatal(err)
	}
}

func alreadyPosted(state State, videoID string) bool {
	for _, id := range state.PostedVideoIDs {
		if id == videoID {
			return true
		}
	}
	return false
}

func getJSON(url string, v any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API取得失敗: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

func addVideo(videos map[string]YouTubeVideo, videoID, title, source, groupID, groupTitle string) {
	if videoID == "" || title == "" {
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
	url := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/channels?part=contentDetails&id=%s&key=%s",
		channelID,
		apiKey,
	)

	var result ChannelsResponse
	if err := getJSON(url, &result); err != nil {
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
		url := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&playlistId=%s&maxResults=50&key=%s&pageToken=%s",
			playlistID,
			apiKey,
			pageToken,
		)

		var result PlaylistItemsResponse
		if err := getJSON(url, &result); err != nil {
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
	pageToken := ""
	videos := make(map[string]YouTubeVideo)

	for {
		url := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/search?part=snippet&channelId=%s&type=video&order=date&maxResults=50&key=%s&pageToken=%s",
			channelID,
			apiKey,
			pageToken,
		)

		var result SearchResponse
		if err := getJSON(url, &result); err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			addVideo(videos, item.ID.VideoID, item.Snippet.Title, "release", "release", "リリース")
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return mapToSlice(videos), nil
}

func fetchVideosFromAllPlaylists(apiKey string) ([]YouTubeVideo, error) {
	pageToken := ""
	allVideos := make(map[string]YouTubeVideo)

	for {
		url := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/playlists?part=snippet&channelId=%s&maxResults=50&key=%s&pageToken=%s",
			channelID,
			apiKey,
			pageToken,
		)

		var result PlaylistsResponse
		if err := getJSON(url, &result); err != nil {
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

	releases, err := fetchVideosFromSearch(apiKey)
	if err != nil {
		return VideoBuckets{}, err
	}

	playlists, err := fetchVideosFromAllPlaylists(apiKey)
	if err != nil {
		return VideoBuckets{}, err
	}

	return VideoBuckets{
		Videos:    videos,
		Releases:  releases,
		Playlists: playlists,
	}, nil
}

func filterCandidates(videos []YouTubeVideo, state State) []YouTubeVideo {
	var candidates []YouTubeVideo

	for _, video := range videos {
		if alreadyPosted(state, video.ID) {
			continue
		}

		if state.LastSource == "release" && video.Source == "release" {
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
		{Weight: 40, Videos: candidates.Videos},
		{Weight: 50, Videos: candidates.Releases},
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

	r := rand.Intn(totalWeight)
	current := 0

	for _, b := range buckets {
		if len(b.Videos) == 0 {
			continue
		}

		current += b.Weight
		if r < current {
			return b.Videos[rand.Intn(len(b.Videos))], true
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
		log.Fatal(err)
	}

	state := loadState()

	candidates := VideoBuckets{
		Videos:    filterCandidates(buckets.Videos, state),
		Releases:  filterCandidates(buckets.Releases, state),
		Playlists: filterCandidates(buckets.Playlists, state),
	}

	rand.Seed(time.Now().UnixNano())
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
		log.Fatal(err)
	}

	ctx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.NewClient(
		cfg.APIAddress,
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := application_apiv1.NewApplicationServiceClient(conn)

	text := "今日の布施明ヽ('∀')ﾉ\n\n" + target.Title + "\n\n" + target.URL

	_, err = client.CreatePost(
		ctx,
		&application_apiv1.CreatePostRequest{
			Text: text,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("投稿成功:", target.Title, "source:", target.Source, "group:", target.GroupTitle)

	state.PostedVideoIDs = append(state.PostedVideoIDs, target.ID)
	state.LastSource = target.Source
	state.LastGroupID = target.GroupID
	saveState(state)
}
