package main

import (
    "context"
    "log"
    "fmt"

    application_apiv1 "github.com/mixigroup/mixi2-application-sample-go/application/apiv1"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    conn, err := grpc.Dial(
        "application.mixi.social:443",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := application_apiv1.NewApplicationServiceClient(conn)
    authCtx := context.Background()

    resp, err := client.CreatePost(authCtx, &application_apiv1.CreatePostRequest{
        Post: &application_apiv1.Post{
            Text: "削除テスト",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("投稿成功")

    _, err = client.DeletePost(authCtx, &application_apiv1.DeletePostRequest{
        PostId: resp.Post.PostId,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("削除成功")
}
