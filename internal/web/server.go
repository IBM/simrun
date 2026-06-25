// Package web implements the REST API, WebSocket hub, and embedded-frontend
// HTTP server.
package web

import (
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/web/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// ServerConfig holds server configuration.
type ServerConfig struct {
	Port           string
	DevMode        bool   // SR_WEB_DEV=1 enables dev proxy to :5173
	DevFrontendURL string // Defaults to http://localhost:5173
}

// Server is the HTTP server for the simrun web UI.
type Server struct {
	router       chi.Router
	hub          *Hub
	config       *ServerConfig
	sessionStore db.SessionStore
}

// NewServer creates a new Server with all routes configured.
func NewServer(handlers *Handlers, packHandlers *PackHandlers, secretHandlers *SecretHandlers, scheduleHandlers *ScheduleHandlers, connectorHandlers *ConnectorHandlers, authHandlers *auth.Handlers, hub *Hub, cfg *ServerConfig, sessionStore db.SessionStore) *Server {
	if cfg.DevFrontendURL == "" {
		cfg.DevFrontendURL = "http://localhost:5173"
	}

	s := &Server{
		router:       chi.NewRouter(),
		hub:          hub,
		config:       cfg,
		sessionStore: sessionStore,
	}

	s.setupRoutes(handlers, packHandlers, secretHandlers, scheduleHandlers, connectorHandlers, authHandlers)
	return s
}

func (s *Server) setupRoutes(handlers *Handlers, packHandlers *PackHandlers, secretHandlers *SecretHandlers, scheduleHandlers *ScheduleHandlers, connectorHandlers *ConnectorHandlers, authHandlers *auth.Handlers) {
	r := s.router

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// CORS middleware for dev mode
	if s.config.DevMode {
		r.Use(corsMiddleware)
	}

	// Health check (public, outside /api; used by load balancers)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth routes
	r.Route("/api/auth", func(r chi.Router) {
		// Public (no session required)
		r.Get("/login", authHandlers.HandleLogin)
		r.Get("/callback", authHandlers.HandleCallback)
		r.Post("/logout", authHandlers.HandleLogout)

		// Authenticated: current user
		if authHandlers.Enabled() {
			r.With(auth.RequireAuth(s.sessionStore)).Get("/me", authHandlers.HandleMe)
		} else {
			r.Get("/me", authHandlers.HandleMe)
		}
	})

	// Authenticated API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(jsonContentType)

		// Apply auth middleware only when OAuth is configured
		if authHandlers.Enabled() {
			r.Use(auth.RequireAuth(s.sessionStore))
		}

		// Assessments (saved definitions; GitHub-Actions "workflow")
		r.Post("/assessments/lint", handlers.HandleLint)
		r.Get("/assessments", handlers.HandleListAssessments)
		r.Post("/assessments", handlers.HandleSaveAssessment)
		r.Get("/assessments/by-name/{name}", handlers.HandleGetAssessmentByName)
		r.Get("/assessments/{id}", handlers.HandleGetAssessment)
		r.Put("/assessments/{id}", handlers.HandleUpdateAssessment)
		r.Delete("/assessments/{id}", handlers.HandleDeleteAssessment)
		r.Get("/assessments/{id}/runs", handlers.HandleListAssessmentRuns)
		r.Get("/assessments/{id}/schedule", scheduleHandlers.HandleGetScheduleByAssessment)

		// Schedules
		r.Get("/schedules", scheduleHandlers.HandleListSchedules)
		r.Post("/schedules", scheduleHandlers.HandleCreateSchedule)
		r.Get("/schedules/{id}", scheduleHandlers.HandleGetSchedule)
		r.Put("/schedules/{id}", scheduleHandlers.HandleUpdateSchedule)
		r.Delete("/schedules/{id}", scheduleHandlers.HandleDeleteSchedule)

		// Runs (executions; created here, read/deleted by id)
		r.Post("/runs", handlers.HandleRun)
		r.Get("/runs", handlers.HandleListRuns)
		r.Get("/runs/{runId}", handlers.HandleGetRun)
		r.Delete("/runs/{runId}", handlers.HandleDeleteRun)
		r.Get("/runs/{runId}/logs", handlers.HandleGetRunLogs)

		// Scenario results
		r.Get("/scenario-results/{id}/collected-logs", handlers.HandleDownloadCollectedLogs)

		// Packs
		r.Get("/packs", packHandlers.HandleListPacks)
		r.Post("/packs/install", packHandlers.HandleInstallPack)
		r.Post("/packs/upload", packHandlers.HandleUploadPack)
		r.Delete("/packs/{name}", packHandlers.HandleDeletePack)
		r.Get("/packs/{name}/manifest", packHandlers.HandleGetManifest)
		r.Get("/packs/{name}/parameters", packHandlers.HandleGetPackParameters)
		r.Put("/packs/{name}/parameters", packHandlers.HandleUpdatePackParameters)

		// Secrets
		r.Get("/secrets", secretHandlers.HandleListSecrets)
		r.Post("/secrets", secretHandlers.HandleSaveSecret)
		r.Get("/secrets/{id}", secretHandlers.HandleGetSecret)
		r.Put("/secrets/{id}", secretHandlers.HandleUpdateSecret)
		r.Delete("/secrets/{id}", secretHandlers.HandleDeleteSecret)

		// Connectors
		r.Get("/connectors", connectorHandlers.HandleListConnectors)
		r.Post("/connectors", connectorHandlers.HandleCreateConnector)
		r.Post("/connectors/test", connectorHandlers.HandleTestConnector)
		r.Get("/connectors/{id}", connectorHandlers.HandleGetConnector)
		r.Put("/connectors/{id}", connectorHandlers.HandleUpdateConnector)
		r.Delete("/connectors/{id}", connectorHandlers.HandleDeleteConnector)
		r.Get("/connectors/{id}/elastic/rules", connectorHandlers.HandleListElasticRules)
		r.Get("/connectors/{id}/elastic/rules/{ruleId}", connectorHandlers.HandleGetElasticRule)

		// Convenience endpoint: auto-detect elastic connector
		r.Get("/elastic/rules", connectorHandlers.HandleListElasticRulesAuto)

		// Rule coverage
		r.Get("/rules/coverage", connectorHandlers.HandleRuleCoverage)

		// Config
		r.Get("/config", handlers.HandleGetConfig)
		r.Put("/config", handlers.HandleUpdateConfig)

		// Version
		r.Get("/version", handlers.HandleVersion)

	})

	// WebSocket (auth checked inline before upgrade)
	r.Get("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		if authHandlers.Enabled() {
			if _, err := auth.CheckSession(s.sessionStore, r); err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
		}
		ServeWS(s.hub, w, r)
	})

	// Frontend: dev proxy to SvelteKit dev server, or serve embedded assets
	if s.config.DevMode {
		target, _ := url.Parse(s.config.DevFrontendURL)
		proxy := httputil.NewSingleHostReverseProxy(target)
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
	} else {
		r.NotFound(spaHandler())
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(":"+s.config.Port, s.router)
}

// Router returns the underlying router. Tests use this to wire the server
// into an httptest.Server without binding a real port.
func (s *Server) Router() http.Handler {
	return s.router
}

// corsMiddleware allows the dev frontend origin with credentials in dev mode.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "http://localhost:5173"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// jsonContentType sets the Content-Type header for API routes.
func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves the embedded frontend assets with SPA fallback.
// Files under _app/immutable/ get long-lived cache headers (hashed filenames).
// index.html is served with no-cache for any path that doesn't match a file.
func spaHandler() http.HandlerFunc {
	stripped, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		// Should never happen — the directory is embedded at compile time.
		panic("failed to create sub filesystem for frontend: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(stripped))

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		// Check if the file exists in the embedded FS.
		f, err := stripped.Open(path)
		if err == nil {
			f.Close()
			// Set cache headers for immutable hashed assets.
			if strings.HasPrefix(r.URL.Path, "/_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA client-side routing.
		indexFile, err := stripped.Open("index.html")
		if err != nil {
			http.Error(w, "frontend not available", http.StatusNotFound)
			return
		}
		indexFile.Close()

		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}
