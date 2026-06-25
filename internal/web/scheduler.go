package web

import (
	"context"
	"sync"
	"time"

	"github.com/IBM/simrun/internal/db"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

// Scheduler manages cron-based scenario execution.
type Scheduler struct {
	scheduleStore   db.ScheduleStore
	assessmentStore db.AssessmentStore
	scenarioService *ScenarioService
	cron            *cron.Cron
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewScheduler creates a new Scheduler.
func NewScheduler(scheduleStore db.ScheduleStore, assessmentStore db.AssessmentStore, scenarioService *ScenarioService) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		scheduleStore:   scheduleStore,
		assessmentStore: assessmentStore,
		scenarioService: scenarioService,
		cron:            cron.New(),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start loads all enabled schedules and starts the cron engine.
func (s *Scheduler) Start() error {
	log.Info("Starting scheduler")

	if err := s.loadSchedules(); err != nil {
		return err
	}

	s.cron.Start()
	log.Info("Scheduler started")
	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	log.Info("Stopping scheduler")
	s.cancel()
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Info("Scheduler stopped")
}

// Reload clears all cron jobs and reloads from database.
func (s *Scheduler) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Info("Reloading schedules")

	for _, entry := range s.cron.Entries() {
		s.cron.Remove(entry.ID)
	}

	if err := s.loadSchedules(); err != nil {
		log.WithError(err).Error("Failed to reload schedules")
	}
}

// loadSchedules fetches enabled schedules from DB and registers cron jobs.
func (s *Scheduler) loadSchedules() error {
	schedules, err := s.scheduleStore.ListEnabled(s.ctx)
	if err != nil {
		return err
	}

	log.Infof("Loading %d enabled schedule(s)", len(schedules))

	for _, schedule := range schedules {
		if err := s.addSchedule(schedule); err != nil {
			log.WithError(err).
				WithField("scheduleId", schedule.ID).
				WithField("cronExpression", schedule.CronExpression).
				Error("Failed to add schedule")
		}
	}

	return nil
}

// addSchedule registers a single schedule as a cron job.
func (s *Scheduler) addSchedule(schedule db.Schedule) error {
	scheduleID := schedule.ID
	scenarioID := schedule.ScenarioID

	_, err := s.cron.AddFunc(schedule.CronExpression, func() {
		s.executeSchedule(scheduleID, scenarioID)
	})

	return err
}

// executeSchedule runs when a cron job fires.
func (s *Scheduler) executeSchedule(scheduleID, scenarioID uuid.UUID) {
	logger := log.WithFields(log.Fields{
		"scheduleId": scheduleID,
		"scenarioId": scenarioID,
	})
	logger.Info("Executing scheduled scenario")

	// Use context.Background() so in-progress runs are not cancelled when the scheduler stops.
	ctx := context.Background()

	schedule, err := s.scheduleStore.Get(ctx, scheduleID)
	if err != nil {
		logger.WithError(err).Error("Failed to load schedule")
		return
	}

	scenario, err := s.assessmentStore.Get(ctx, scenarioID)
	if err != nil {
		logger.WithError(err).Error("Failed to load scenario for scheduled run")
		return
	}

	scheduleName := scenario.Name + " (scheduled)"

	_, err = s.scenarioService.Run(ctx, scenarioID, &RunOptions{
		Parallelism:  schedule.Parallelism,
		ScheduleID:   &scheduleID,
		ScheduleName: &scheduleName,
		CreatedBy:    "system",
	})
	if err != nil {
		logger.WithError(err).Error("Failed to execute scheduled scenario")
		return
	}

	if err := s.scheduleStore.UpdateLastRun(ctx, scheduleID, time.Now()); err != nil {
		logger.WithError(err).Warn("Failed to update last_run_at")
	}
}
