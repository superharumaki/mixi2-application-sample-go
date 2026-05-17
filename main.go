package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"math/rand"
	"os"
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

type State struct {
	Index int `json:"index"`
}

const stateFile = "state.json"

// state読み込み
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

// state保存
func saveState(s State) {
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(stateFile, data, 0644)
}

func main() {

	// 設定（環境変数ベースに統一）
	cfg := config.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		APIAddress:   os.Getenv("API_ADDRESS"),
		StreamAddress: os.Getenv("STREAM_ADDRESS"),
		TokenURL:     os.Getenv("TOKEN_URL"),
	}

	// 認証
	authenticator, err := auth.NewAuthenticator(
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.TokenURL,
	)
	if err != nil {
		log.Fatal(err)
	}

	// gRPC接続
	apiConn, err := grpc.Dial(
		cfg.APIAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer apiConn.Close()

	client := application_apiv1.NewApplicationServiceClient(apiConn)

	// 認証コンテキスト
	authCtx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// 曲リスト
	songs := []Song{
		{
			Title: "ウンポ・ダモーレ・アンケ・ペル・メ",
			URL:   "https://www.youtube.com/watch?v=dFKAKUeoVT8",
		},
		{
			Title: "ふるき友心の唄",
			URL:   "https://www.youtube.com/watch?v=i6JEvM2yzCQ",
		},
		{
			Title: "とおる君",
			URL:   "https://www.youtube.com/watch?v=5EXwW0FcbVo",
		},
		{
			Title: "帰りこぬ愛の唄",
			URL:   "https://www.youtube.com/watch?v=up04--yqmUY",
		},
	}

	// 乱数（将来用）
	rand.Seed(time.Now().UnixNano())

	// state読み込み
	state := loadState()

	// 安全チェック
	if state.Index < 0 || state.Index >= len(songs) {
		state.Index = 0
	}

	// 曲選択
	song := songs[state.Index]

	// インデックス更新
	state.Index++
	if state.Index >= len(songs) {
		state.Index = 0
	}

	saveState(state)

	// 投稿テキスト
	text := "今日の布施明🎤\n\n" +
		song.Title + "\n\n" +
		song.URL + "\n\n" +
		"#布施明"

	// 投稿
	_, err = client.CreatePost(authCtx, &application_apiv1.CreatePostRequest{
		Text: text,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("投稿成功:", song.Title)
}
