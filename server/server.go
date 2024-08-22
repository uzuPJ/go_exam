package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	todo "go_exam"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/google/uuid"
)

func main() {
	srv := handler.NewDefaultServer(todo.NewExecutableSchema(todo.New()))
	srv.SetRecoverFunc(func(ctx context.Context, err any) (userMessage error) {
		// send this panic somewhere
		log.Print(err)
		debug.PrintStack()
		return errors.New("user message on panic")
	})

	sessionIDCtxKey := "sessionID"

	// リクエスト内容をログ出力
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		// 処理を追えるようセッション用途でuuidを生成
		uuid, err := uuid.NewRandom()
		if err != nil {
			panic(err)
		}
		// レスポンスでもidを追えるようcontextに追加
		newCtx := context.WithValue(ctx, sessionIDCtxKey, uuid)

		oc := graphql.GetOperationContext(ctx)
		requestLog := fmt.Sprintf("SessionID:%s,OperationName:%s,Query:%s", uuid, oc.OperationName, oc.RawQuery)
		// ログ出力
		log.Println(requestLog)
		res := next(newCtx)
		return res
	})

	// レスポンス内容をログ出力
	srv.AroundRootFields(func(ctx context.Context, next graphql.RootResolver) graphql.Marshaler {
		res := next(ctx)
		defer func() {
			var b bytes.Buffer
			res.MarshalGQL(&b)
			sessionID := ctx.Value(sessionIDCtxKey)
			responseLog := fmt.Sprintf("SessionID:%s,ResponseContents:%s", sessionID, b.String())
			// ログ出力
			log.Println(responseLog)
		}()
		return res
	})

	http.Handle("/", playground.Handler("Todo", "/query"))
	http.Handle("/query", srv)
	log.Fatal(http.ListenAndServe(":8081", nil))
}
