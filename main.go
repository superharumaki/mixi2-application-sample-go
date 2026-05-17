package main

import (
	"context"
	"crypto/tls"
	"log"
	"os"

	"://github.com"
	"://github.com"
	application_apiv1 "://github.com"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// ご提示いただいた消したい過去のポストIDの一覧
	deletePostIDs := []string{
		"f7d27b27-9194-493d-9362-52465940b367",
		"76defd81-91b2-42b4-bf71-d20ce6c86ecc",
		"11bd1936-304b-4c4a-96a8-7eb6d8b7e04d",
		"5a1a9968-3d4a-41ec-9f19-7e021a8974bb",
		"d7f69944-7c6c-4203-9282-5d132f1b2bf1",
		"a94517c7-97c4-4acc-a459-2c7c68acdf61",
		"aae2aa57-3e23-4769-a02e-2d7960eb9005",
		"4241e64b-87e4-4bba-8362-938bb8eff51b",
		"15e56a67-0f31-4a50-a702-ce102ed8b927",
		"c7d7aebd-6db5-4c91-a645-1414e658451e",
		"b6e9d2dd-9803-4c5c-9c2e-f71b2da8dc79",
		"6a288018-917f-4b83-9180-1dbeaa5d7efc",
		"6fab78c0-f1c6-47dc-8da4-cf35ce13f9b9",
		"4715a51a-e1bd-40cf-abef-527ca4bb551f",
		"40f91140-a2cd-4fc7-be4d-a8fb9419e9a1",
		"e5d96ab0-faf7-41f4-ad69-ef2a809e98c4",
		"18ed73ef-6a63-46e3-8a30-d49ac9136292",
	}

	// 設定（環境変数ベース）
	cfg := config.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		APIAddress:   os.Getenv("API_ADDRESS"),
		TokenURL:     os.Getenv("TOKEN_URL"),
	}

	// 認証
	authenticator, err := auth.NewAuthenticator(cfg.ClientID, cfg.ClientSecret, cfg.TokenURL)
	if err != nil {
		log.Fatal(err)
	}

	// gRPC接続
	apiConn, err := grpc.Dial(cfg.APIAddress, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		log.Fatal(err)
	}
	defer apiConn.Close()

	client := application_apiv1.NewApplicationServiceClient(apiConn)
	authCtx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// リストにあるIDを順番にループして全て削除する
	log.Printf("合計 %d 件のテストポストを順番に削除します...", len(deletePostIDs))
	
	successCount := 0
	failCount := 0

	for i, id := range deletePostIDs {
		log.Printf("[%d/%d] 削除要求中: %s ...", i+1, len(deletePostIDs), id)
		_, err = client.DeletePost(authCtx, &application_apiv1.DeletePostRequest{
			PostId: id,
		})
		if err != nil {
			log.Printf("【失敗】ID: %s の削除に失敗しました（既に消えているか、権限がない可能性があります）: %v", id, err)
			failCount++
			continue
		}
		log.Printf("【成功】ID: %s を削除しました。", id)
		successCount++
	}

	log.Printf("削除処理が終了しました。結果: 成功 %d 件 / 失敗 %d 件\n", successCount, failCount)
	log.Println("※元の「曲を投稿するbot」に戻すには、以前の main.go のコードを再度貼り付け直してください。")
}
