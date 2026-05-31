package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ScheduleHandlers provides REST handlers for schedule management.
type ScheduleHandlers struct {
	scheduleStore db.ScheduleStore
	scenarioStore db.ScenarioStore
	scheduler     *Scheduler
}

// NewScheduleHandlers creates a new ScheduleHandlers instance.
func NewScheduleHandlers(scheduleStore db.ScheduleStore, scenarioStore db.ScenarioStore, scheduler *Scheduler) *ScheduleHandlers {
	return &ScheduleHandlers{
		scheduleStore: scheduleStore,
		scenarioStore: scenarioStore,
		scheduler:     scheduler,
	}
}

// HandleListSchedules handles GET /api/schedules
func (h *ScheduleHandlers) HandleListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.scheduleStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, schedules)
}

// HandleGetSchedule handles GET /api/schedules/{id}
func (h *ScheduleHandlers) HandleGetSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule ID")
		return
	}

	schedule, err := h.scheduleStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	writeJSON(w, http.StatusOK, schedule)
}

// HandleGetScheduleByScenario handles GET /api/scenarios/{scenarioId}/schedule
func (h *ScheduleHandlers) HandleGetScheduleByScenario(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := uuid.Parse(chi.URLParam(r, "scenarioId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenario ID")
		return
	}

	schedule, err := h.scheduleStore.GetByScenarioID(r.Context(), scenarioID)
	if err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	writeJSON(w, http.StatusOK, schedule)
}

// HandleCreateSchedule handles POST /api/schedules
func (h *ScheduleHandlers) HandleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	scenarioID, err := uuid.Parse(req.ScenarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenario ID")
		return
	}

	if _, err := h.scenarioStore.Get(r.Context(), scenarioID); err != nil {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}

	if err := validateCronExpression(req.CronExpression); err != nil {
		writeError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
		return
	}

	parallelism := req.Parallelism
	if parallelism <= 0 {
		parallelism = 10
	}

	schedule, err := h.scheduleStore.Create(r.Context(), scenarioID, req.CronExpression, req.Enabled, parallelism, getUserEmail(r))
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeError(w, http.StatusConflict, "schedule already exists for this scenario")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.scheduler.Reload()

	writeJSON(w, http.StatusCreated, schedule)
}

// HandleUpdateSchedule handles PUT /api/schedules/{id}
func (h *ScheduleHandlers) HandleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule ID")
		return
	}

	var req UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateCronExpression(req.CronExpression); err != nil {
		writeError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
		return
	}

	parallelism := req.Parallelism
	if parallelism <= 0 {
		parallelism = 10
	}

	if err := h.scheduleStore.Update(r.Context(), id, req.CronExpression, req.Enabled, parallelism, getUserEmail(r)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.scheduler.Reload()

	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteSchedule handles DELETE /api/schedules/{id}
func (h *ScheduleHandlers) HandleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule ID")
		return
	}

	if err := h.scheduleStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.scheduler.Reload()

	w.WriteHeader(http.StatusNoContent)
}

func validateCronExpression(expr string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(expr)
	return err
}
