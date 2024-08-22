### todo app

This is the simplest example of a graphql server.

to run this server

```bash
go run ./server/server.go
```

and open http://localhost:8081 in your browser

## 設計意図

### 課題 1

- 削除操作で、指定した todo の ID がリストに存在していたら delete してメモリの配列を再セットし、結果を true で返す
- 削除操作で、指定した todo の ID がリストに存在していなかったら、結果を false で返すだけとする

### 課題 2

- subscription ではユーザ ID を仮置きで引数に設定し、それをキーにチャンネルを作成。今回はユーザ管理までの実装は行っていないので、ユーザに関係なくメモリで管理している todo の内容を返すようにした。
- todo リストの内容に変更があったらリストを全て返すようにしている。こうすることでフロントエンド側では初回に todo リストを取得し、あとは subscription で配信されたリストを置き換えればよいということになる。
- 今回は todo リストの件数が少ない前提として全件返す方針とした。リストのデータが多くなれば、個別に変更内容をメッセージにして返すなどの考慮は必要になってくる。

### 課題 3

- ログ出力の対象として、GraphQL のリクエストのクエリ内容とレスポンスの中身とした。実装方針としては gqlgen の Middleware を使用する。
- リクエストとレスポンスをセットで追えるうに、今回は仮置きで uuid を発行して一緒に出力するようにした。リクエストのログ出力時に uuid を発行しレスポンスでも追えるよう、context にセットした。
- ログの出力は今回は簡易的な実装で`log.Println`を使用した。
