package main

import (
	"context"
	"crypto/tls"
	"log"
	"math/rand"
	"time"

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

func main() {

	cfg := config.GetConfig()

	authenticator, err := auth.NewAuthenticator(
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.TokenURL,
	)

	if err != nil {
		log.Fatal(err)
	}

	apiConn, err := grpc.NewClient(
		cfg.APIAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)

	if err != nil {
		log.Fatal(err)
	}

	defer apiConn.Close()

	client := application_apiv1.NewApplicationServiceClient(apiConn)

	authCtx, err := authenticator.AuthorizedContext(context.Background())

	if err != nil {
		log.Fatal(err)
	}

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
			Title: "積木の部屋",
			URL:   "https://www.youtube.com/results?search_query=布施明+積木の部屋",
		},
		{
			Title: "MY WAY",
			URL:   "https://www.youtube.com/results?search_query=布施明+MY+WAY",
		},
	}

	rand.Seed(time.Now().UnixNano())

	song := songs[rand.Intn(len(songs))]

	text := "今日の布施明🎤\n\n" +
		song.Title +
		"\n\n▶ " +
		song.URL +
		"\n\n#布施明"

	_, err = client.CreatePost(authCtx, &application_apiv1.CreatePostRequest{
		Text: text,
	})

	if err != nil {
		log.Fatal(err)
	}

	log.Println("投稿成功:", song.Title)
}
