// Package fakes provides in-memory implementations of every db.*Store
// interface. They satisfy the same contracts as the production stores but
// hold state in maps; pair with testutil/testserver for handler tests, or
// inject directly into services that need stores without Postgres.
package fakes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IBM/simrun/internal/config"
	"github.com/IBM/simrun/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Stores bundles every fake store. Use New() to construct a wired-up bundle.
type Stores struct {
	Run       *RunStore
	Scenario  *ScenarioStore
	Pack      *PackStore
	Secret    *SecretStore
	Connector *ConnectorStore
	Config    *ConfigStore
	Schedule  *ScheduleStore
	Session   *SessionStore
}

// New returns a bundle with every store initialized to an empty map.
func New() *Stores {
	return &Stores{
		Run:       &RunStore{runs: map[uuid.UUID]*db.Run{}, results: map[uuid.UUID]*db.ScenarioResult{}},
		Scenario:  &ScenarioStore{scenarios: map[uuid.UUID]*db.SavedScenario{}},
		Pack:      &PackStore{packs: map[string]*db.Pack{}},
		Secret:    &SecretStore{secrets: map[uuid.UUID]*db.SecretGroup{}},
		Connector: &ConnectorStore{connectors: map[uuid.UUID]*db.Connector{}},
		Config:    &ConfigStore{data: map[string]json.RawMessage{}},
		Schedule:  &ScheduleStore{schedules: map[uuid.UUID]*db.Schedule{}},
		Session:   &SessionStore{sessions: map[string]*db.AuthSession{}},
	}
}

// ---- RunStore ----

type RunStore struct {
	mu      sync.Mutex
	runs    map[uuid.UUID]*db.Run
	results map[uuid.UUID]*db.ScenarioResult
}

var _ db.RunStore = (*RunStore)(nil)

func (s *RunStore) Create(_ context.Context, run *db.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *run
	s.runs[cp.ID] = &cp
	return nil
}

func (s *RunStore) Get(_ context.Context, id uuid.UUID) (*db.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *r
	return &cp, nil
}

func (s *RunStore) List(_ context.Context, filters db.ListRunsFilters, limit, offset int) (db.RunPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := make([]db.Run, 0, len(s.runs))
	for _, r := range s.runs {
		if !matchesRunFilters(r, filters) {
			continue
		}
		all = append(all, *r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	total := len(all)
	start := min(offset, total)
	end := min(start+limit, total)
	return db.RunPage{Runs: all[start:end], Total: total}, nil
}

func matchesRunFilters(r *db.Run, f db.ListRunsFilters) bool {
	if f.Name != "" {
		if r.ScenarioName == nil || !strings.Contains(strings.ToLower(*r.ScenarioName), strings.ToLower(f.Name)) {
			return false
		}
	}
	if len(f.Types) > 0 {
		if r.ScenarioType == nil || !slices.Contains(f.Types, *r.ScenarioType) {
			return false
		}
	}
	if f.Since != nil && r.CreatedAt.Before(*f.Since) {
		return false
	}
	if f.ScenarioID != nil {
		if r.ScenarioID == nil || *r.ScenarioID != *f.ScenarioID {
			return false
		}
	}
	return true
}

func (s *RunStore) Update(_ context.Context, id uuid.UUID, status string, total, succeeded, failed int, endTime *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return pgx.ErrNoRows
	}
	r.Status = status
	r.Total = total
	r.Succeeded = succeeded
	r.Failed = failed
	r.EndTime = endTime
	return nil
}

func (s *RunStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runs, id)
	for rid, r := range s.results {
		if r.RunID == id {
			delete(s.results, rid)
		}
	}
	return nil
}

func (s *RunStore) AddScenarioResult(_ context.Context, runID uuid.UUID, result *db.ScenarioResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *result
	cp.RunID = runID
	if cp.ID == uuid.Nil {
		cp.ID = uuid.New()
	}
	s.results[cp.ID] = &cp
	return nil
}

func (s *RunStore) GetScenarioResults(_ context.Context, runID uuid.UUID) ([]db.ScenarioResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []db.ScenarioResult{}
	for _, r := range s.results {
		if r.RunID == runID {
			out = append(out, *r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *RunStore) GetScenarioResult(_ context.Context, id uuid.UUID) (*db.ScenarioResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.results[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *r
	return &cp, nil
}

func (s *RunStore) CompleteRun(_ context.Context, id uuid.UUID, endTime *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return pgx.ErrNoRows
	}
	r.EndTime = endTime
	r.Status = "completed"
	return nil
}

func (s *RunStore) CreateScenarioStatus(_ context.Context, runID uuid.UUID, name string) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := uuid.New()
	s.results[id] = &db.ScenarioResult{
		ID:        id,
		RunID:     runID,
		Name:      name,
		Status:    "running",
		CreatedAt: time.Now(),
	}
	return id, nil
}

func (s *RunStore) UpdateScenarioPhase(_ context.Context, id uuid.UUID, phase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.results[id]
	if !ok {
		return pgx.ErrNoRows
	}
	r.Phase = &phase
	return nil
}

func (s *RunStore) CompleteScenarioResult(_ context.Context, id uuid.UUID, result *db.ScenarioResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.results[id]; !ok {
		return pgx.ErrNoRows
	}
	cp := *result
	cp.ID = id
	s.results[id] = &cp
	return nil
}

func (s *RunStore) IncrementRunCounters(_ context.Context, id uuid.UUID, successDelta, failDelta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return pgx.ErrNoRows
	}
	r.Succeeded += successDelta
	r.Failed += failDelta
	return nil
}

func (s *RunStore) GetLatestAssertionResults(_ context.Context) ([]db.LatestAssertionResult, error) {
	return []db.LatestAssertionResult{}, nil
}

// All returns a snapshot of all runs (test-only convenience).
func (s *RunStore) All() []db.Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]db.Run, 0, len(s.runs))
	for _, r := range s.runs {
		out = append(out, *r)
	}
	return out
}

// ---- ScenarioStore ----

type ScenarioStore struct {
	mu        sync.Mutex
	scenarios map[uuid.UUID]*db.SavedScenario
}

var _ db.ScenarioStore = (*ScenarioStore)(nil)

func (s *ScenarioStore) Save(_ context.Context, name, scenarioType, yaml, createdBy string) (*db.SavedScenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	sc := &db.SavedScenario{
		ID:        uuid.New(),
		Name:      name,
		Type:      scenarioType,
		YAML:      yaml,
		CreatedBy: createdBy,
		UpdatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.scenarios[sc.ID] = sc
	cp := *sc
	return &cp, nil
}

func (s *ScenarioStore) Get(_ context.Context, id uuid.UUID) (*db.SavedScenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.scenarios[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *sc
	return &cp, nil
}

func (s *ScenarioStore) List(_ context.Context, filters db.ListScenariosFilters, limit, offset int) (db.ScenarioPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := make([]db.SavedScenario, 0, len(s.scenarios))
	for _, sc := range s.scenarios {
		if !scenarioMatchesFilters(*sc, filters) {
			continue
		}
		all = append(all, *sc)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].UpdatedAt.After(all[j].UpdatedAt) })
	total := len(all)
	if offset >= total {
		return db.ScenarioPage{Scenarios: []db.SavedScenario{}, Total: total}, nil
	}
	end := min(offset+limit, total)
	page := append([]db.SavedScenario(nil), all[offset:end]...)
	return db.ScenarioPage{Scenarios: page, Total: total}, nil
}

func (s *ScenarioStore) ListAll(_ context.Context) ([]db.SavedScenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]db.SavedScenario, 0, len(s.scenarios))
	for _, sc := range s.scenarios {
		out = append(out, *sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func scenarioMatchesFilters(sc db.SavedScenario, f db.ListScenariosFilters) bool {
	if f.Name != "" && !strings.Contains(strings.ToLower(sc.Name), strings.ToLower(f.Name)) {
		return false
	}
	if len(f.Types) > 0 && !slices.Contains(f.Types, sc.Type) {
		return false
	}
	if f.Since != nil && sc.UpdatedAt.Before(*f.Since) {
		return false
	}
	return true
}

func (s *ScenarioStore) Update(_ context.Context, id uuid.UUID, name, scenarioType, yaml, updatedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.scenarios[id]
	if !ok {
		return pgx.ErrNoRows
	}
	sc.Name = name
	sc.Type = scenarioType
	sc.YAML = yaml
	sc.UpdatedBy = updatedBy
	sc.UpdatedAt = time.Now()
	return nil
}

// SetUpdatedAt backdates a scenario's UpdatedAt for tests exercising the
// `since` filter; no-op if the scenario is unknown.
func (s *ScenarioStore) SetUpdatedAt(id uuid.UUID, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sc, ok := s.scenarios[id]; ok {
		sc.UpdatedAt = t
	}
}

func (s *ScenarioStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.scenarios, id)
	return nil
}

// ---- PackStore ----

type PackStore struct {
	mu    sync.Mutex
	packs map[string]*db.Pack
}

var _ db.PackStore = (*PackStore)(nil)

func (s *PackStore) Upsert(_ context.Context, pack *db.Pack, installedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cp := *pack
	if existing, ok := s.packs[pack.Name]; ok {
		cp.ID = existing.ID
		cp.CreatedAt = existing.CreatedAt
	} else {
		if cp.ID == uuid.Nil {
			cp.ID = uuid.New()
		}
		cp.CreatedAt = now
	}
	cp.InstalledBy = installedBy
	cp.UpdatedAt = now
	s.packs[pack.Name] = &cp
	return nil
}

func (s *PackStore) Get(_ context.Context, name string) (*db.Pack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packs[name]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *p
	return &cp, nil
}

func (s *PackStore) List(_ context.Context) ([]db.Pack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]db.Pack, 0, len(s.packs))
	for _, p := range s.packs {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *PackStore) Delete(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.packs, name)
	return nil
}

func (s *PackStore) UpdateParameters(_ context.Context, name string, parameters map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packs[name]
	if !ok {
		return pgx.ErrNoRows
	}
	p.Parameters = parameters
	p.UpdatedAt = time.Now()
	return nil
}

// ---- SecretStore ----

type SecretStore struct {
	mu      sync.Mutex
	secrets map[uuid.UUID]*db.SecretGroup
}

var _ db.SecretStore = (*SecretStore)(nil)

func (s *SecretStore) Save(_ context.Context, name, description string, entries json.RawMessage, createdBy string) (*db.SecretGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	sg := &db.SecretGroup{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Entries:     entries,
		CreatedBy:   createdBy,
		UpdatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.secrets[sg.ID] = sg
	cp := *sg
	return &cp, nil
}

func (s *SecretStore) Get(_ context.Context, id uuid.UUID) (*db.SecretGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sg, ok := s.secrets[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *sg
	return &cp, nil
}

func (s *SecretStore) List(_ context.Context) ([]db.SecretGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]db.SecretGroup, 0, len(s.secrets))
	for _, sg := range s.secrets {
		out = append(out, *sg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *SecretStore) Update(_ context.Context, id uuid.UUID, name, description string, entries json.RawMessage, updatedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sg, ok := s.secrets[id]
	if !ok {
		return pgx.ErrNoRows
	}
	sg.Name = name
	sg.Description = description
	sg.Entries = entries
	sg.UpdatedBy = updatedBy
	sg.UpdatedAt = time.Now()
	return nil
}

func (s *SecretStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, id)
	return nil
}

// ---- ConnectorStore ----

type ConnectorStore struct {
	mu         sync.Mutex
	connectors map[uuid.UUID]*db.Connector
}

var _ db.ConnectorStore = (*ConnectorStore)(nil)

func (s *ConnectorStore) Save(_ context.Context, name, connectorType, description string, secretGroupID *uuid.UUID, cfg json.RawMessage, isDefault bool, createdBy string) (*db.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if isDefault {
		for _, c := range s.connectors {
			if c.Type == connectorType {
				c.IsDefault = false
			}
		}
	}
	now := time.Now()
	c := &db.Connector{
		ID:            uuid.New(),
		Name:          name,
		Type:          connectorType,
		Description:   description,
		SecretGroupID: secretGroupID,
		Config:        cfg,
		Enabled:       true,
		IsDefault:     isDefault,
		CreatedBy:     createdBy,
		UpdatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.connectors[c.ID] = c
	cp := *c
	return &cp, nil
}

func (s *ConnectorStore) Get(_ context.Context, id uuid.UUID) (*db.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.connectors[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *c
	return &cp, nil
}

func (s *ConnectorStore) GetByName(_ context.Context, name string) (*db.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.connectors {
		if c.Name == name {
			cp := *c
			return &cp, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (s *ConnectorStore) GetDefault(_ context.Context, connectorType string) (*db.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.connectors {
		if c.Type == connectorType && c.IsDefault && c.Enabled {
			cp := *c
			return &cp, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (s *ConnectorStore) List(_ context.Context) ([]db.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]db.Connector, 0, len(s.connectors))
	for _, c := range s.connectors {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *ConnectorStore) Update(_ context.Context, id uuid.UUID, name, description string, secretGroupID *uuid.UUID, cfg json.RawMessage, enabled bool, isDefault bool, updatedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.connectors[id]
	if !ok {
		return pgx.ErrNoRows
	}
	if isDefault {
		for otherID, other := range s.connectors {
			if otherID != id && other.Type == c.Type {
				other.IsDefault = false
			}
		}
	}
	c.Name = name
	c.Description = description
	c.SecretGroupID = secretGroupID
	c.Config = cfg
	c.Enabled = enabled
	c.IsDefault = isDefault
	c.UpdatedBy = updatedBy
	c.UpdatedAt = time.Now()
	return nil
}

func (s *ConnectorStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connectors, id)
	return nil
}

// All returns a snapshot of all connectors (test-only convenience).
func (s *ConnectorStore) All() []db.Connector {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]db.Connector, 0, len(s.connectors))
	for _, c := range s.connectors {
		out = append(out, *c)
	}
	return out
}

// ---- ConfigStore ----

type ConfigStore struct {
	mu   sync.Mutex
	data map[string]json.RawMessage
}

var _ db.ConfigStore = (*ConfigStore)(nil)

func (s *ConfigStore) Get(_ context.Context, key string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key], nil
}

func (s *ConfigStore) Set(_ context.Context, key string, value json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *ConfigStore) GetAll(_ context.Context) (map[string]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]json.RawMessage, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out, nil
}

// GetAppConfig assembles the typed AppConfig from the stored JSON keys. When a
// key is missing or malformed, falls back to that field's default — matches
// the production store's tolerant parsing.
func (s *ConfigStore) GetAppConfig(_ context.Context) (config.AppConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := config.DefaultAppConfig()
	if raw, ok := s.data["parallelism"]; ok {
		var v int
		if err := json.Unmarshal(raw, &v); err == nil && v > 0 {
			out.Parallelism = v
		}
	}
	if raw, ok := s.data["terraform_version"]; ok {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil {
			out.TerraformVersion = v
		}
	}
	if raw, ok := s.data["pack_logs_enabled"]; ok {
		var v bool
		if err := json.Unmarshal(raw, &v); err == nil {
			out.PackLogsEnabled = v
		}
	}
	if raw, ok := s.data["ssh_logging_enabled"]; ok {
		var v bool
		if err := json.Unmarshal(raw, &v); err == nil {
			out.SSHLoggingEnabled = v
		}
	}
	return out, nil
}

func (s *ConfigStore) UpdateAppConfig(_ context.Context, c config.AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range map[string]any{
		"parallelism":         c.Parallelism,
		"terraform_version":   c.TerraformVersion,
		"pack_logs_enabled":   c.PackLogsEnabled,
		"ssh_logging_enabled": c.SSHLoggingEnabled,
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		s.data[k] = raw
	}
	return nil
}

// ---- ScheduleStore ----

type ScheduleStore struct {
	mu        sync.Mutex
	schedules map[uuid.UUID]*db.Schedule
}

var _ db.ScheduleStore = (*ScheduleStore)(nil)

func (s *ScheduleStore) Create(_ context.Context, scenarioID uuid.UUID, cronExpr string, enabled bool, parallelism int, createdBy string) (*db.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	sched := &db.Schedule{
		ID:             uuid.New(),
		ScenarioID:     scenarioID,
		CronExpression: cronExpr,
		Enabled:        enabled,
		Parallelism:    parallelism,
		CreatedBy:      createdBy,
		UpdatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.schedules[sched.ID] = sched
	cp := *sched
	return &cp, nil
}

func (s *ScheduleStore) Get(_ context.Context, id uuid.UUID) (*db.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sched, ok := s.schedules[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *sched
	return &cp, nil
}

func (s *ScheduleStore) GetByScenarioID(_ context.Context, scenarioID uuid.UUID) (*db.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sched := range s.schedules {
		if sched.ScenarioID == scenarioID {
			cp := *sched
			return &cp, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (s *ScheduleStore) List(_ context.Context) ([]db.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]db.Schedule, 0, len(s.schedules))
	for _, sched := range s.schedules {
		out = append(out, *sched)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *ScheduleStore) ListEnabled(ctx context.Context) ([]db.Schedule, error) {
	all, _ := s.List(ctx)
	out := make([]db.Schedule, 0, len(all))
	for _, sched := range all {
		if sched.Enabled {
			out = append(out, sched)
		}
	}
	return out, nil
}

func (s *ScheduleStore) Update(_ context.Context, id uuid.UUID, cronExpr string, enabled bool, parallelism int, updatedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sched, ok := s.schedules[id]
	if !ok {
		return pgx.ErrNoRows
	}
	sched.CronExpression = cronExpr
	sched.Enabled = enabled
	sched.Parallelism = parallelism
	sched.UpdatedBy = updatedBy
	sched.UpdatedAt = time.Now()
	return nil
}

func (s *ScheduleStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.schedules, id)
	return nil
}

func (s *ScheduleStore) UpdateLastRun(_ context.Context, id uuid.UUID, lastRunAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sched, ok := s.schedules[id]
	if !ok {
		return pgx.ErrNoRows
	}
	sched.LastRunAt = &lastRunAt
	return nil
}

// ---- SessionStore ----

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*db.AuthSession
}

var _ db.SessionStore = (*SessionStore)(nil)

func (s *SessionStore) Create(_ context.Context, email, name, picture string, ttl time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := randomSessionID()
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.sessions[id] = &db.AuthSession{
		ID:        id,
		Email:     email,
		Name:      name,
		Picture:   picture,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	return id, nil
}

func (s *SessionStore) Get(_ context.Context, sessionID string) (*db.AuthSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, pgx.ErrNoRows
	}
	cp := *sess
	return &cp, nil
}

func (s *SessionStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *SessionStore) DeleteExpired(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
	return nil
}

func randomSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
