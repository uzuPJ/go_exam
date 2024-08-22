//go:generate go run ../../testdata/gqlgen.go

package todo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/99designs/gqlgen/graphql"
)

var (
	you  = &User{ID: 1, Name: "You"}
	them = &User{ID: 2, Name: "Them"}
)

type ckey string

func getUserId(ctx context.Context) int {
	if id, ok := ctx.Value(ckey("userId")).(int); ok {
		return id
	}
	return you.ID
}

func New() Config {
	c := Config{
		Resolvers: &resolvers{
			todos: []*Todo{
				{ID: 1, Text: "A todo not to forget", Done: false, owner: you},
				{ID: 2, Text: "This is the most important", Done: false, owner: you},
				{ID: 3, Text: "Somebody else's todo", Done: true, owner: them},
				{ID: 4, Text: "Please do this or else", Done: false, owner: you},
			},
			lastID:      4,
			subscribers: map[string]chan<- []*Todo{},
			mutex:       sync.Mutex{},
		},
	}
	c.Directives.HasRole = func(ctx context.Context, obj any, next graphql.Resolver, role Role) (any, error) {
		switch role {
		case RoleAdmin:
			// No admin for you!
			return nil, nil
		case RoleOwner:
			ownable, isOwnable := obj.(Ownable)
			if !isOwnable {
				return nil, errors.New("obj cant be owned")
			}

			if ownable.Owner().ID != getUserId(ctx) {
				return nil, errors.New("you don't own that")
			}
		}

		return next(ctx)
	}
	c.Directives.User = func(ctx context.Context, obj any, next graphql.Resolver, id int) (any, error) {
		return next(context.WithValue(ctx, ckey("userId"), id))
	}
	return c
}

type resolvers struct {
	todos       []*Todo
	lastID      int
	subscribers map[string]chan<- []*Todo
	mutex       sync.Mutex
}

func (r *resolvers) MyQuery() MyQueryResolver {
	return (*QueryResolver)(r)
}

func (r *resolvers) MyMutation() MyMutationResolver {
	return (*MutationResolver)(r)
}

func (r *resolvers) MySubscription() MySubscriptionResolver {
	return (*SubscriptionResolver)(r)
}

type QueryResolver resolvers

func (r *QueryResolver) Todo(ctx context.Context, id int) (*Todo, error) {
	time.Sleep(220 * time.Millisecond)

	if id == 666 {
		panic("critical failure")
	}

	for _, todo := range r.todos {
		if todo.ID == id {
			return todo, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *QueryResolver) LastTodo(ctx context.Context) (*Todo, error) {
	if len(r.todos) == 0 {
		return nil, errors.New("not found")
	}
	return r.todos[len(r.todos)-1], nil
}

func (r *QueryResolver) Todos(ctx context.Context) ([]*Todo, error) {
	return r.todos, nil
}

type MutationResolver resolvers

func (r *MutationResolver) CreateTodo(ctx context.Context, todo TodoInput) (*Todo, error) {
	newID := r.id()

	newTodo := &Todo{
		ID:    newID,
		Text:  todo.Text,
		owner: you,
	}

	if todo.Done != nil {
		newTodo.Done = *todo.Done
	}

	r.todos = append(r.todos, newTodo)

	deliveryTodos(r.todos, r.subscribers, &r.mutex)
	return newTodo, nil
}

func (r *MutationResolver) UpdateTodo(ctx context.Context, id int, changes map[string]any) (*Todo, error) {
	var affectedTodo *Todo

	for i := 0; i < len(r.todos); i++ {
		if r.todos[i].ID == id {
			affectedTodo = r.todos[i]
			break
		}
	}

	if affectedTodo == nil {
		return nil, nil
	}

	err := mapstructure.Decode(changes, affectedTodo)
	if err != nil {
		panic(err)
	}

	deliveryTodos(r.todos, r.subscribers, &r.mutex)
	return affectedTodo, nil
}

func (r *MutationResolver) DeleteTodo(ctx context.Context, id int) (bool, error) {
	var deletedTodos []*Todo
	findResult := false

	for i := 0; i < len(r.todos); i++ {
		if r.todos[i].ID == id {
			findResult = true
		} else {
			deletedTodos = append(deletedTodos, r.todos[i])
		}
	}

	if !findResult {
		return false, nil
	}

	r.todos = deletedTodos
	deliveryTodos(r.todos, r.subscribers, &r.mutex)
	return true, nil
}

func (r *MutationResolver) id() int {
	r.lastID++
	return r.lastID
}

type SubscriptionResolver resolvers

func (r *SubscriptionResolver) SubscriptionTodo(ctx context.Context, userID int) (<-chan []*Todo, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	userIDStr := strconv.Itoa(userID)
	if _, ok := r.subscribers[userIDStr]; ok {
		err := fmt.Errorf("`%s` has already been subscribed.", userIDStr)
		log.Print(err.Error())
		return nil, err
	}

	// チャンネルを作成し、リストに登録
	ch := make(chan []*Todo, 1)
	r.subscribers[userIDStr] = ch
	log.Printf("`%s` has been subscribed!", userIDStr)

	// コネクションが終了したら、このチャンネルを削除する
	go func() {
		<-ctx.Done()
		r.mutex.Lock()
		delete(r.subscribers, userIDStr)
		r.mutex.Unlock()
		log.Printf("`%s` has been unsubscribed.", userIDStr)
	}()
	return ch, nil
}

// 更新操作をされたらsubscriberにTODOリストを配信する
func deliveryTodos(todos []*Todo,
	subscribers map[string]chan<- []*Todo,
	mutex *sync.Mutex) {

	mutex.Lock()
	for _, ch := range subscribers {
		ch <- todos
	}
	mutex.Unlock()
}
