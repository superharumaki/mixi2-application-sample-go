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
