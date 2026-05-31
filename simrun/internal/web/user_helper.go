package web

import (
	"net/http"

	"github.com/IBM/simrun/simrun/internal/web/auth"
)

// getUserEmail extracts the authenticated user's email from the request context.
// Returns "anonymous" when authentication is disabled.
func getUserEmail(r *http.Request) string {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return "anonymous"
	}
	return user.Email
}
