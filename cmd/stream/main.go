package main

import (
	"context"
	"crypto/tls"
	"log"
	"math/rand"
	"time"

	"github.com/mixigroup/mixi2-application-sample-go/config"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {

	cfg := config.GetConfig()

	apiConn, err := grpc.NewClient(
		cfg.APIAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)

	if err != nil {
		log.Fatal(err)
	}

	defer apiConn.Close()

	client := application_apiv1.NewApplicationServiceClient(apiConn)

	authCtx := context.Background()

	songs := []string{
		"シクラメンのかほり",
		"君は薔薇より美しい",
		"積木の部屋",
		"MY WAY",
	}

	rand.Seed(time.Now().UnixNano())

	song := songs[rand.Intn(len(songs))]

	text := "今日の布施明🎤\n\n" + song + "\n\n#布施明"

	_, err = client.CreatePost(authCtx, &application_apiv1.CreatePostRequest{
		Text: text,
	})

	if err != nil {
		log.Fatal(err)
	}

	log.Println("投稿成功:", song)
}
