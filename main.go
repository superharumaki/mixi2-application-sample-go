resp, err := client.CreatePost(authCtx, &application_apiv1.CreatePostRequest{
    Post: &application_apiv1.Post{
        Text: "削除テスト",
    },
})
if err != nil {
    log.Fatal("create error:", err)
}

if resp == nil || resp.Post == nil {
    log.Fatal("create response is nil")
}

postID := resp.Post.PostId
fmt.Println("投稿成功 PostID:", postID)

_, err = client.DeletePost(authCtx, &application_apiv1.DeletePostRequest{
    PostId: postID,
})
if err != nil {
    log.Fatal("delete error:", err)
}

fmt.Println("削除成功")
