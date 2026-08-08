package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultOpsHostHealthPath     = "/run/sub2api-ops/host-health.json"
	defaultOpsHostHealthMaxAge   = 90 * time.Second
	opsHostHealthPathEnvVar      = "SUB2API_HOST_HEALTH_PATH"
	opsHostHealthUnavailableText = "host health snapshot is not available"
)

type OpsHostHealthService struct {
	path   string
	maxAge time.Duration
	now    func() time.Time
}

type OpsHostHealthSnapshot struct {
	Available     bool                       `json:"available"`
	Status        string                     `json:"status"`
	Message       string                     `json:"message,omitempty"`
	CollectedAt   time.Time                  `json:"collected_at,omitempty"`
	AgeSeconds    int64                      `json:"age_seconds,omitempty"`
	Stale         bool                       `json:"stale"`
	LoadAverage   OpsHostLoadAverage         `json:"load_average"`
	CPU           OpsHostCPU                 `json:"cpu"`
	Memory        OpsHostMemory              `json:"memory"`
	TopContainers []OpsHostTopContainer      `json:"top_containers,omitempty"`
	TopProcesses  []OpsHostTopProcess        `json:"top_processes,omitempty"`
	Diagnosis     string                     `json:"diagnosis,omitempty"`
	Raw           map[string]json.RawMessage `json:"-"`
}

type OpsHostLoadAverage struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

type OpsHostCPU struct {
	UsagePercent float64 `json:"usage_percent"`
	High         bool    `json:"high"`
}

type OpsHostMemory struct {
	AvailableMB int64 `json:"available_mb"`
	SwapUsedMB  int64 `json:"swap_used_mb"`
}

type OpsHostTopContainer struct {
	Name    string  `json:"name"`
	CPUPerc float64 `json:"cpu_percent"`
	Memory  string  `json:"memory"`
	PIDs    int64   `json:"pids"`
}

type OpsHostTopProcess struct {
	PID     int64   `json:"pid"`
	Command string  `json:"command"`
	CPUPerc float64 `json:"cpu_percent"`
	RSSMB   int64   `json:"rss_mb"`
}

func NewOpsHostHealthService(path string) *OpsHostHealthService {
	path = strings.TrimSpace(path)
	if path == "" {
		path = strings.TrimSpace(os.Getenv(opsHostHealthPathEnvVar))
	}
	if path == "" {
		path = defaultOpsHostHealthPath
	}
	return &OpsHostHealthService{
		path:   path,
		maxAge: defaultOpsHostHealthMaxAge,
		now:    time.Now,
	}
}

func (s *OpsHostHealthService) GetSnapshot(ctx context.Context) (*OpsHostHealthSnapshot, error) {
	if s == nil {
		return unavailableOpsHostHealthSnapshot("missing"), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return unavailableOpsHostHealthSnapshot("missing"), nil
		}
		return nil, fmt.Errorf("read host health snapshot: %w", err)
	}

	var snapshot OpsHostHealthSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil, fmt.Errorf("decode host health snapshot: %w", err)
	}
	if snapshot.CollectedAt.IsZero() {
		snapshot.Available = false
		snapshot.Status = "invalid"
		snapshot.Message = "host health snapshot has no collected_at"
		return &snapshot, nil
	}

	now := s.now()
	age := now.Sub(snapshot.CollectedAt)
	if age < 0 {
		age = 0
	}
	snapshot.AgeSeconds = int64(age.Seconds())
	snapshot.Available = true
	if age > s.maxAge {
		snapshot.Stale = true
		snapshot.Status = "stale"
		if snapshot.Message == "" {
			snapshot.Message = "host health snapshot is stale"
		}
	} else {
		snapshot.Stale = false
		snapshot.Status = "ok"
	}
	return &snapshot, nil
}

func unavailableOpsHostHealthSnapshot(status string) *OpsHostHealthSnapshot {
	return &OpsHostHealthSnapshot{
		Available: false,
		Status:    status,
		Message:   opsHostHealthUnavailableText,
	}
}

func (s *OpsService) GetHostHealth(ctx context.Context) (*OpsHostHealthSnapshot, error) {
	return NewOpsHostHealthService("").GetSnapshot(ctx)
}
