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
}

type YouTubeVideo struct {
	ID    string
	Title string
	URL   string
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

func addVideo(videos map[string]YouTubeVideo, videoID, title string) {
	if videoID == "" || title == "" {
		return
	}

	if _, exists := videos[videoID]; exists {
		return
	}

	videos[videoID] = YouTubeVideo{
		ID:    videoID,
		Title: title,
		URL:   "https://youtu.be/" + videoID,
	}
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

func fetchVideosFromPlaylist(apiKey, playlistID string, videos map[string]YouTubeVideo) error {
	pageToken := ""

	for {
		url := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&playlistId=%s&maxResults=50&key=%s&pageToken=%s",
			playlistID,
			apiKey,
			pageToken,
		)

		var result PlaylistItemsResponse
		if err := getJSON(url, &result); err != nil {
			return err
		}

		for _, item := range result.Items {
			addVideo(videos, item.Snippet.ResourceID.VideoID, item.Snippet.Title)
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return nil
}

func fetchVideosFromSearch(apiKey string, videos map[string]YouTubeVideo) error {
	pageToken := ""

	for {
		url := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/search?part=snippet&channelId=%s&type=video&order=date&maxResults=50&key=%s&pageToken=%s",
			channelID,
			apiKey,
			pageToken,
		)

		var result SearchResponse
		if err := getJSON(url, &result); err != nil {
			return err
		}

		for _, item := range result.Items {
			addVideo(videos, item.ID.VideoID, item.Snippet.Title)
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return nil
}

func fetchVideosFromAllPlaylists(apiKey string, videos map[string]YouTubeVideo) error {
	pageToken := ""

	for {
		url := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/playlists?part=snippet&channelId=%s&maxResults=50&key=%s&pageToken=%s",
			channelID,
			apiKey,
			pageToken,
		)

		var result PlaylistsResponse
		if err := getJSON(url, &result); err != nil {
			return err
		}

		for _, playlist := range result.Items {
			if playlist.ID == "" {
				continue
			}
			if err := fetchVideosFromPlaylist(apiKey, playlist.ID, videos); err != nil {
				log.Println("再生リスト取得スキップ:", playlist.Snippet.Title, err)
			}
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return nil
}

func fetchAllVideos(apiKey string) ([]YouTubeVideo, error) {
	videosMap := make(map[string]YouTubeVideo)

	uploadsPlaylistID, err := getUploadsPlaylistID(apiKey)
	if err != nil {
		return nil, err
	}

	if err := fetchVideosFromPlaylist(apiKey, uploadsPlaylistID, videosMap); err != nil {
		return nil, err
	}

	if err := fetchVideosFromSearch(apiKey, videosMap); err != nil {
		return nil, err
	}

	if err := fetchVideosFromAllPlaylists(apiKey, videosMap); err != nil {
		return nil, err
	}

	videos := make([]YouTubeVideo, 0, len(videosMap))
	for _, video := range videosMap {
		videos = append(videos, video)
	}

	return videos, nil
}

func main() {
	cfg := config.GetConfig()

	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		log.Fatal("YOUTUBE_API_KEY missing value")
	}

	videos, err := fetchAllVideos(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	state := loadState()

	var candidates []YouTubeVideo
	for _, video := range videos {
		if !alreadyPosted(state, video.ID) {
			candidates = append(candidates, video)
		}
	}

	if len(candidates) == 0 {
		log.Println("未投稿の公式動画がありません")
		return
	}

	rand.Seed(time.Now().UnixNano())
	target := candidates[rand.Intn(len(candidates))]

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

	log.Println("投稿成功:", target.Title)

	state.PostedVideoIDs = append(state.PostedVideoIDs, target.ID)
	saveState(state)
}
