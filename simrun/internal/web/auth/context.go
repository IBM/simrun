package auth

import "context"

type contextKey int

const userContextKey contextKey = iota

// WithUser adds a User to the request context.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext extracts the User from the request context.
// Returns nil if no user is set (i.e. unauthenticated).
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}
