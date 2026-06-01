package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/version"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handlers provides REST handlers for scenarios, runs, config, and version.
type Handlers struct {
	scenarioService *ScenarioService
	scenarioStore   db.ScenarioStore
	runStore        db.RunStore
	configStore     db.ConfigStore
	scheduler       *Scheduler
	dataDir         string
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(ss *ScenarioService, scenarioStore db.ScenarioStore, runStore db.RunStore, configStore db.ConfigStore, scheduler *Scheduler, dataDir string) *Handlers {
	return &Handlers{
		scenarioService: ss,
		scenarioStore:   scenarioStore,
		runStore:        runStore,
		configStore:     configStore,
		scheduler:       scheduler,
		dataDir:         dataDir,
	}
}

// HandleLint handles POST /api/scenarios/lint
func (h *Handlers) HandleLint(w http.ResponseWriter, r *http.Request) {
	var req LintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.scenarioService.Lint([]byte(req.YAML))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleRun handles POST /api/scenarios/run
func (h *Handlers) HandleRun(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	scenarioID, err := uuid.Parse(req.ScenarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenarioId")
		return
	}

	var timeout time.Duration
	if req.Timeout != "" {
		timeout, err = time.ParseDuration(req.Timeout)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timeout format; use Go duration (e.g. '10m', '30s')")
			return
		}
	}

	runID, err := h.scenarioService.Run(r.Context(), scenarioID, &RunOptions{
		Parallelism:   req.Parallelism,
		CreatedBy:     getUserEmail(r),
		ExploreMode:   req.ExploreMode,
		CleanupAlerts: req.CleanupAlerts,
		Timeout:       timeout,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, RunResponse{RunID: runID})
}

// HandleListScenarios handles GET /api/scenarios.
// Pagination: page (default 1), per_page (default 50, clamped to [1, 100]).
// Filters: name (ILIKE %name% on scenario name), type (repeatable —
// e.g. ?type=standard&type=explore), since (Go duration like "24h" — returns
// scenarios updated in that window).
func (h *Handlers) HandleListScenarios(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r, 50, 100)
	filters, err := parseScenarioFilters(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.scenarioStore.List(r.Context(), filters, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scenarios": res.Scenarios,
		"total":     res.Total,
		"page":      page,
		"perPage":   perPage,
	})
}

// parseScenarioFilters extracts filter query params for HandleListScenarios.
func parseScenarioFilters(r *http.Request) (db.ListScenariosFilters, error) {
	q := r.URL.Query()
	f := db.ListScenariosFilters{Name: q.Get("name")}
	for _, t := range q["type"] {
		if !validScenarioTypes[t] {
			return db.ListScenariosFilters{}, fmt.Errorf("invalid type %q (allowed: standard, explore, collect)", t)
		}
		f.Types = append(f.Types, t)
	}
	if s := q.Get("since"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return db.ListScenariosFilters{}, fmt.Errorf("invalid since %q (expected Go duration like '24h')", s)
		}
		t := time.Now().Add(-d)
		f.Since = &t
	}
	return f, nil
}

// HandleSaveScenario handles POST /api/scenarios
func (h *Handlers) HandleSaveScenario(w http.ResponseWriter, r *http.Request) {
	var req SaveScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	scenarioType, err := normalizeScenarioType(req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	saved, err := h.scenarioStore.Save(r.Context(), req.Name, scenarioType, req.YAML, getUserEmail(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, saved)
}

// HandleGetScenario handles GET /api/scenarios/{id}
func (h *Handlers) HandleGetScenario(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenario ID")
		return
	}

	scenario, err := h.scenarioStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}

	writeJSON(w, http.StatusOK, scenario)
}

// HandleUpdateScenario handles PUT /api/scenarios/{id}
func (h *Handlers) HandleUpdateScenario(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenario ID")
		return
	}

	var req SaveScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	scenarioType, err := normalizeScenarioType(req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.scenarioStore.Update(r.Context(), id, req.Name, scenarioType, req.YAML, getUserEmail(r)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteScenario handles DELETE /api/scenarios/{id}
func (h *Handlers) HandleDeleteScenario(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenario ID")
		return
	}

	if err := h.scenarioStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Cascade delete removes the schedule row, but the cron job is still registered.
	// Reload the scheduler to remove the orphaned cron entry.
	if h.scheduler != nil {
		h.scheduler.Reload()
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListRuns handles GET /api/runs.
// Pagination: page (default 1), per_page (default 50, clamped to [1, 100]).
// Filters: name (ILIKE %name% on saved scenario name), type (repeatable —
// e.g. ?type=standard&type=explore), since (Go duration like "24h" — returns
// runs created in that window).
func (h *Handlers) HandleListRuns(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r, 50, 100)
	filters, err := parseRunFilters(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.runStore.List(r.Context(), filters, perPage, (page-1)*perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runs":    res.Runs,
		"total":   res.Total,
		"page":    page,
		"perPage": perPage,
	})
}

// parseRunFilters extracts filter query params for HandleListRuns.
func parseRunFilters(r *http.Request) (db.ListRunsFilters, error) {
	q := r.URL.Query()
	f := db.ListRunsFilters{Name: q.Get("name")}
	for _, t := range q["type"] {
		if !validScenarioTypes[t] {
			return db.ListRunsFilters{}, fmt.Errorf("invalid type %q (allowed: standard, explore, collect)", t)
		}
		f.Types = append(f.Types, t)
	}
	if s := q.Get("since"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return db.ListRunsFilters{}, fmt.Errorf("invalid since %q (expected Go duration like '24h')", s)
		}
		t := time.Now().Add(-d)
		f.Since = &t
	}
	if s := q.Get("scenario_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return db.ListRunsFilters{}, fmt.Errorf("invalid scenario_id %q", s)
		}
		f.ScenarioID = &id
	}
	return f, nil
}

// parsePagination reads `page` and `per_page` query params, applying defaults and clamps.
func parsePagination(r *http.Request, defaultPerPage, maxPerPage int) (page, perPage int) {
	page = 1
	perPage = defaultPerPage
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 {
			perPage = min(v, maxPerPage)
		}
	}
	return page, perPage
}

// HandleGetRun handles GET /api/runs/{runId}
func (h *Handlers) HandleGetRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "runId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	run, err := h.runStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	scenarioResults, err := h.runStore.GetScenarioResults(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"run":       run,
		"scenarios": scenarioResults,
	})
}

// HandleDeleteRun handles DELETE /api/runs/{runId}
func (h *Handlers) HandleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "runId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := h.runStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Clean up log file (best-effort)
	DeleteRunLog(h.dataDir, id.String())

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetRunLogs handles GET /api/runs/{runId}/logs
func (h *Handlers) HandleGetRunLogs(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	if _, err := uuid.Parse(runID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	entries, err := ReadRunLog(h.dataDir, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

// HandleDownloadCollectedLogs handles GET /api/scenario-results/{id}/collected-logs
func (h *Handlers) HandleDownloadCollectedLogs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenario result ID")
		return
	}

	result, err := h.runStore.GetScenarioResult(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "scenario result not found")
		return
	}

	if result.CollectedLogPath == nil || *result.CollectedLogPath == "" {
		writeError(w, http.StatusNotFound, "no collected logs available for this scenario")
		return
	}

	// Validate the path ends with .ndjson to guard against serving arbitrary files
	logPath := filepath.Clean(*result.CollectedLogPath)
	if filepath.Ext(logPath) != ".ndjson" {
		writeError(w, http.StatusForbidden, "invalid log file path")
		return
	}

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "collected log file not found on disk")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to open collected log file")
		}
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file metadata")
		return
	}

	filename := filepath.Base(logPath)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	_, _ = io.Copy(w, file)
}

// HandleGetConfig handles GET /api/config
func (h *Handlers) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configStore.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// HandleUpdateConfig handles PUT /api/config
func (h *Handlers) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.configStore.Set(r.Context(), req.Key, req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleVersion handles GET /api/version
func (h *Handlers) HandleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, VersionResponse{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,
		GoVersion: runtime.Version(),
	})
}

// normalizeScenarioType validates and defaults the scenario type.
func normalizeScenarioType(typ string) (string, error) {
	if typ == "" {
		return ScenarioTypeStandard, nil
	}
	if !validScenarioTypes[typ] {
		return "", fmt.Errorf("type must be '%s', '%s', or '%s'", ScenarioTypeStandard, ScenarioTypeExplore, ScenarioTypeCollect)
	}
	return typ, nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}
