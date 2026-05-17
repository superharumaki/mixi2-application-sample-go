package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/mixigroup/mixi2-application-sample-go/config"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Song struct {
	Title string
	URL   string
}

type State struct {
	Index int `json:"index"`
}

const stateFile = "state.json"

func loadState() State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return State{Index: 0}
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{Index: 0}
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

func main() {
	songs := []Song{
		{
			Title: "シクラメンのかほり",
			URL:   "https://www.youtube.com/results?search_query=布施明+シクラメンのかほり",
		},
		{
			Title: "君は薔薇より美しい",
			URL:   "https://www.youtube.com/results?search_query=布施明+君は薔薇より美しい",
		},
		{
			Title: "マイ・ウェイ",
			URL:   "https://www.youtube.com/results?search_query=布施明+マイウェイ",
		},
	}

	cfg := config.GetConfig()

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

	state := loadState()

	if state.Index < 0 || state.Index >= len(songs) {
		state.Index = 0
	}

	song := songs[state.Index]

	text := "今日の布施明\n" +
		song.Title + "\n" +
		song.URL

	_, err = client.CreatePost(
		ctx,
		&application_apiv1.CreatePostRequest{
			Text: text,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("投稿成功:", song.Title)

	state.Index++

	if state.Index >= len(songs) {
		state.Index = 0
	}

	saveState(state)
}
