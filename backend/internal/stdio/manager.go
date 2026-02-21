package stdio

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/reflow/gateway/internal/database"
	"github.com/rs/zerolog/log"
)

// Manager manages the lifecycle of STDIO MCP processes.
type Manager struct {
	mu        sync.Mutex
	processes map[string]*Process // subjectKey → process
	repo      *database.Repository

	idleTTL      time.Duration
	maxLifetime  time.Duration
	gcInterval   time.Duration
	maxProcesses int

	stopGC chan struct{}
}

// ManagerConfig holds configuration for the STDIO manager.
type ManagerConfig struct {
	IdleTTL      time.Duration
	MaxLifetime  time.Duration
	GCInterval   time.Duration
	MaxProcesses int
}

// NewManager creates a new STDIO process manager.
func NewManager(repo *database.Repository, cfg ManagerConfig) *Manager {
	if cfg.IdleTTL == 0 {
		cfg.IdleTTL = 30 * time.Minute
	}
	if cfg.MaxLifetime == 0 {
		cfg.MaxLifetime = 24 * time.Hour
	}
	if cfg.GCInterval == 0 {
		cfg.GCInterval = 1 * time.Minute
	}
	if cfg.MaxProcesses == 0 {
		cfg.MaxProcesses = 100
	}

	m := &Manager{
		processes:    make(map[string]*Process),
		repo:         repo,
		idleTTL:      cfg.IdleTTL,
		maxLifetime:  cfg.MaxLifetime,
		gcInterval:   cfg.GCInterval,
		maxProcesses: cfg.MaxProcesses,
		stopGC:       make(chan struct{}),
	}

	// Cleanup any stale DB records from previous runs
	if repo != nil {
		count, err := repo.CleanupMCPInstances(context.Background())
		if err != nil {
			log.Warn().Err(err).Msg("Failed to cleanup stale MCP instances")
		} else if count > 0 {
			log.Info().Int64("count", count).Msg("Cleaned up stale MCP instances from previous run")
		}
	}

	go m.gcLoop()
	return m
}

// GetOrCreate returns an existing process for the given subject key, or creates a new one.
func (m *Manager) GetOrCreate(ctx context.Context, subjectKey string, cfg ProcessConfig) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for existing alive process
	if proc, ok := m.processes[subjectKey]; ok {
		if proc.IsAlive() {
			return proc, nil
		}
		// Dead process, clean up
		delete(m.processes, subjectKey)
		log.Debug().Str("subject_key", subjectKey).Msg("Removed dead STDIO process")
	}

	// Check capacity
	if len(m.processes) >= m.maxProcesses {
		return nil, fmt.Errorf("max STDIO processes reached (%d)", m.maxProcesses)
	}

	// Start new process
	proc, err := NewProcess(cfg)
	if err != nil {
		return nil, err
	}

	m.processes[subjectKey] = proc

	// Record in database
	if m.repo != nil {
		if err := m.repo.UpsertMCPInstance(ctx, uuid.Nil, subjectKey, proc.PID()); err != nil {
			log.Warn().Err(err).Str("subject_key", subjectKey).Msg("Failed to record MCP instance in DB")
		}
	}

	return proc, nil
}

// GetOrCreateForTarget is a convenience that builds the ProcessConfig from a target and env.
func (m *Manager) GetOrCreateForTarget(ctx context.Context, subjectKey string, target *database.Target, env []string) (*Process, error) {
	cfg := ProcessConfig{
		Command:    target.Command,
		Args:       target.Args,
		Env:        env,
		SubjectKey: subjectKey,
		TargetName: target.Name,
	}
	return m.GetOrCreate(ctx, subjectKey, cfg)
}

// gcLoop periodically cleans up idle and expired processes.
func (m *Manager) gcLoop() {
	ticker := time.NewTicker(m.gcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.gc()
		case <-m.stopGC:
			return
		}
	}
}

func (m *Manager) gc() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, proc := range m.processes {
		if !proc.IsAlive() {
			delete(m.processes, key)
			log.Debug().Str("subject_key", key).Msg("GC: removed dead STDIO process")
			if m.repo != nil {
				m.repo.StopMCPInstance(context.Background(), uuid.Nil, key)
			}
			continue
		}

		idleFor := now.Sub(proc.LastUsed())
		if idleFor > m.idleTTL {
			log.Info().
				Str("subject_key", key).
				Str("idle_for", idleFor.String()).
				Msg("GC: stopping idle STDIO process")
			proc.Close()
			delete(m.processes, key)
			if m.repo != nil {
				m.repo.StopMCPInstance(context.Background(), uuid.Nil, key)
			}
		}
	}
}

// Shutdown gracefully stops all processes.
func (m *Manager) Shutdown() {
	close(m.stopGC)

	m.mu.Lock()
	defer m.mu.Unlock()

	log.Info().Int("count", len(m.processes)).Msg("Shutting down all STDIO processes")

	for key, proc := range m.processes {
		proc.Close()
		if m.repo != nil {
			m.repo.StopMCPInstance(context.Background(), uuid.Nil, key)
		}
		delete(m.processes, key)
	}
}

// Stats returns statistics about running processes.
type Stats struct {
	Total int `json:"total"`
	Alive int `json:"alive"`
	Max   int `json:"max"`
}

func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()

	alive := 0
	for _, proc := range m.processes {
		if proc.IsAlive() {
			alive++
		}
	}

	return Stats{
		Total: len(m.processes),
		Alive: alive,
		Max:   m.maxProcesses,
	}
}

// ComputeSubjectKey computes the subject key for isolation routing.
func ComputeSubjectKey(target *database.Target, userID string, role string, groups []string) string {
	switch target.IsolationBoundary {
	case "per_user":
		h := sha256.Sum256([]byte(userID + ":" + target.ID.String()))
		return fmt.Sprintf("user:%x", h[:8])
	case "per_role":
		h := sha256.Sum256([]byte(role + ":" + target.ID.String()))
		return fmt.Sprintf("role:%x", h[:8])
	case "per_group":
		group := ""
		if len(groups) > 0 {
			sorted := make([]string, len(groups))
			copy(sorted, groups)
			sort.Strings(sorted)
			group = strings.Join(sorted, ",")
		}
		h := sha256.Sum256([]byte(group + ":" + target.ID.String()))
		return fmt.Sprintf("group:%x", h[:8])
	default: // "shared"
		return "shared:" + target.ID.String()
	}
}
