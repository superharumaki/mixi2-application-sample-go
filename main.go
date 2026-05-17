package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/mixigroup/mixi2-application-sample-go/config"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const stateFile = "state.json"
const youtubeFeedURL = "https://www.youtube.com/feeds/videos.xml?channel_id=UCtEbxUxhVwEwFuNzTjrpzOg"

type Feed struct {
	Entries []Entry `xml:"entry"`
}

type Entry struct {
	VideoID string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	Title   string `xml:"title"`
	Link    Link   `xml:"link"`
}

type Link struct {
	Href string `xml:"href,attr"`
}

type State struct {
	PostedVideoIDs []string `json:"posted_video_ids"`
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

func fetchVideos() ([]Entry, error) {
	resp, err := http.Get(youtubeFeedURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("YouTube RSS取得失敗: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var feed Feed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	return feed.Entries, nil
}

func main() {
	cfg := config.GetConfig()

	videos, err := fetchVideos()
	if err != nil {
		log.Fatal(err)
	}

	if len(videos) == 0 {
		log.Fatal("YouTube公式チャンネルの動画が見つかりませんでした")
	}

	state := loadState()

	var target *Entry

	// RSSは新しい順なので、古いものから順番に投稿する
	for i := len(videos) - 1; i >= 0; i-- {
		video := videos[i]
		if !alreadyPosted(state, video.VideoID) {
			target = &video
			break
		}
	}

	if target == nil {
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

	text := "今日の布施明 公式動画\n" +
		target.Title + "\n" +
		target.Link.Href

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

	state.PostedVideoIDs = append(state.PostedVideoIDs, target.VideoID)
	saveState(state)
}
