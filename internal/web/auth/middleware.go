package auth

import (
	"encoding/json"
	"net/http"

	"github.com/IBM/simrun/internal/db"
)

const sessionCookieName = "simrun_session"

// RequireAuth returns middleware that validates the session cookie
// and injects the authenticated User into the request context.
// Unauthenticated requests receive a 401 JSON response.
func RequireAuth(sessionStore db.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			sess, err := sessionStore.Get(r.Context(), cookie.Value)
			if err != nil {
				// Clear invalid/expired cookie
				http.SetCookie(w, &http.Cookie{
					Name:   sessionCookieName,
					Path:   "/",
					MaxAge: -1,
				})
				writeAuthError(w, http.StatusUnauthorized, "session expired")
				return
			}

			user := &User{
				Email:   sess.Email,
				Name:    sess.Name,
				Picture: sess.Picture,
			}
			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CheckSession validates the session cookie without blocking.
// Used for endpoints like WebSocket that need auth but not the middleware chain.
func CheckSession(sessionStore db.SessionStore, r *http.Request) (*User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, db.ErrSessionNotFound
	}

	sess, err := sessionStore.Get(r.Context(), cookie.Value)
	if err != nil {
		return nil, err
	}

	return &User{
		Email:   sess.Email,
		Name:    sess.Name,
		Picture: sess.Picture,
	}, nil
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
