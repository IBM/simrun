// Package auth provides Google OAuth login and session-cookie middleware for the
// web API.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/IBM/simrun/internal/db"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	stateCookieName = "oauth_state"
)

// Config holds authentication configuration.
type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	AllowedDomain      string
	SessionTTL         time.Duration
}

// Handlers provides OAuth2 authentication HTTP handlers.
type Handlers struct {
	sessionStore  db.SessionStore
	oauthConfig   *oauth2.Config
	allowedDomain string
	sessionTTL    time.Duration
	enabled       bool
}

// NewHandlers creates auth handlers. If client ID or secret are empty,
// authentication is disabled and handlers return 503.
func NewHandlers(sessionStore db.SessionStore, cfg Config) *Handlers {
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		log.Warn("Google OAuth not configured (SR_GOOGLE_CLIENT_ID or SR_GOOGLE_CLIENT_SECRET missing) — auth disabled")
		return &Handlers{
			sessionStore: sessionStore,
			sessionTTL:   cfg.SessionTTL,
			enabled:      false,
		}
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	h := &Handlers{
		sessionStore:  sessionStore,
		oauthConfig:   oauthConfig,
		allowedDomain: cfg.AllowedDomain,
		sessionTTL:    cfg.SessionTTL,
		enabled:       true,
	}

	log.Infof("Google OAuth enabled (domain: %s)", cfg.AllowedDomain)
	return h
}

// Enabled returns whether OAuth authentication is configured.
func (h *Handlers) Enabled() bool {
	return h.enabled
}

// HandleLogin redirects the user to Google's OAuth2 consent screen.
func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		http.Error(w, "authentication not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := generateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Build redirect URL from the current request
	redirectURL := buildRedirectURL(r)
	oauthCfg := *h.oauthConfig
	oauthCfg.RedirectURL = redirectURL

	url := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// HandleCallback processes the OAuth2 callback from Google.
func (h *Handlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		http.Error(w, "authentication not configured", http.StatusServiceUnavailable)
		return
	}

	// Verify state for CSRF protection
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(stateCookie.Value)) != 1 {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   stateCookieName,
		Path:   "/",
		MaxAge: -1,
	})

	// Check for OAuth error
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		log.Warnf("OAuth error: %s", errMsg)
		http.Error(w, "authentication denied", http.StatusForbidden)
		return
	}

	// Exchange authorization code for token
	redirectURL := buildRedirectURL(r)
	oauthCfg := *h.oauthConfig
	oauthCfg.RedirectURL = redirectURL

	code := r.URL.Query().Get("code")
	token, err := oauthCfg.Exchange(r.Context(), code)
	if err != nil {
		log.Errorf("OAuth token exchange failed: %v", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	// Fetch user info from Google
	userInfo, err := fetchGoogleUserInfo(r.Context(), &oauthCfg, token)
	if err != nil {
		log.Errorf("Failed to fetch user info: %v", err)
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	// Enforce domain restriction
	if h.allowedDomain != "" && !strings.HasSuffix(userInfo.Email, "@"+h.allowedDomain) {
		log.Warnf("Login rejected for %s (allowed domain: %s)", userInfo.Email, h.allowedDomain)
		http.Error(w, fmt.Sprintf("only @%s accounts are allowed", h.allowedDomain), http.StatusForbidden)
		return
	}

	// Create server-side session
	sessionID, err := h.sessionStore.Create(r.Context(), userInfo.Email, userInfo.Name, userInfo.Picture, h.sessionTTL)
	if err != nil {
		log.Errorf("Failed to create session: %v", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	log.Infof("User logged in: %s", userInfo.Email)

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(h.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to app root
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// HandleLogout clears the session and cookie.
func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = h.sessionStore.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Path:   "/",
		MaxAge: -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

// HandleMe returns the currently authenticated user.
// When auth is disabled, returns an anonymous user so the frontend can proceed.
func (h *Handlers) HandleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil && !h.enabled {
		user = &User{
			Email: "anonymous",
			Name:  "Anonymous",
		}
	}
	if user == nil {
		writeAuthError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// fetchGoogleUserInfo calls Google's userinfo API using the OAuth token.
func fetchGoogleUserInfo(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned status %d", resp.StatusCode)
	}

	var info GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}
	return &info, nil
}

// buildRedirectURL constructs the OAuth callback URL from the current request.
func buildRedirectURL(r *http.Request) string {
	// Allow explicit override via env var
	if base := os.Getenv("SR_WEB_URL"); base != "" {
		return strings.TrimRight(base, "/") + "/api/auth/callback"
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Check X-Forwarded-Proto for reverse proxy setups
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	return fmt.Sprintf("%s://%s/api/auth/callback", scheme, host)
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
