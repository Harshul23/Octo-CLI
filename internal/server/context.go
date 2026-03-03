package server

import (
	"context"
)

type contextKey string

const userContextKey contextKey = "user"

// withUser adds a user to the request context
func withUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// userFromContext extracts the user from the request context
func userFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}
