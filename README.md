# mixi2-application-sample-go

mixi2 に音楽動画を自動投稿する Bot です。

## 機能

* YouTube動画をランダム取得
* 同じ動画を連続投稿しない
* 同じ種類（release など）や同じ再生リストからの連投を避ける制御があります。
* 投稿済みを `state.json` で管理
* GitHub Actions で毎日自動投稿

---

# 自動投稿時間

GitHub Actions の cron で毎日実行しています。

```yaml
cron: '17 21 * * *'
```

UTC基準なので、日本時間では6時17分ごろ実行されます。

---

# 必要なSecrets

GitHub：

Settings
→ Secrets and variables
→ Actions
→ New repository secret

必要なもの：

```text
CLIENT_ID
CLIENT_SECRET
YOUTUBE_API_KEY
```

---

# 手動実行

GitHub：

Actions
→ Post Bot
→ Run workflow

---

# 投稿済み管理

`state.json` で投稿済み動画を管理しています。
投稿済み動画や直前のカテゴリ情報を保持します。

例：

```json
{
  "posted_video_ids": [],
  "last_source": "",
  "last_group_id": ""
}
```

---

# 投稿リセット

最初から再投稿したい場合：

`state.json` を以下の内容に戻します。

```json
{
  "posted_video_ids": [],
  "last_source": "",
  "last_group_id": ""
}
```

その後：

```bash
git add state.json
git commit -m "reset posted videos"
git push
```

---

# 投稿削除方法

mixi2 の投稿IDが分かっている場合、ターミナルから削除できます。

## delete.go を作成

```go
package main

import (
	"context"
	"crypto/tls"
	"log"
	"os"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	authenticator, err := auth.NewAuthenticator(
		os.Getenv("CLIENT_ID"),
		os.Getenv("CLIENT_SECRET"),
		os.Getenv("TOKEN_URL"),
	)
	if err != nil {
		log.Fatal(err)
	}

	authCtx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.Dial(
		os.Getenv("API_ADDRESS"),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := application_apiv1.NewApplicationServiceClient(conn)

	postID := "ここに投稿ID"

	_, err = client.DeletePost(authCtx, &application_apiv1.DeletePostRequest{
		PostId: postID,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("削除成功")
}
```

---

## 環境変数設定

```bash
export CLIENT_ID=xxxxxxxx
export CLIENT_SECRET=xxxxxxxx
export TOKEN_URL=https://application-auth.mixi.social/oauth2/token
export API_ADDRESS=application-api.mixi.social:443
```

---

## 実行

```bash
go run delete.go
```

成功すると：

```text
削除成功
```

と表示されます。

---

## プレビュー実行

投稿せずに内容だけ確認したい場合（ローカル確認用）:

```bash
PREVIEW=1 go run .
```

---

# 注意

* YouTube API quota 超過時は投稿失敗します
* 長すぎるタイトルは自動的に「…」で短縮されます
* 投稿失敗時は state.json を更新しません
* workflow の同時実行防止あり
* GitHub Actionsでビルドチェックを行っています。
* push競合対策あり
* GitHub Actions の cron は UTC 基準です
