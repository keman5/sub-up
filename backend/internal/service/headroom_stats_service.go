package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

var ErrHeadroomStatsDisabled = errors.New("headroom stats disabled")

type HeadroomStatsService struct {
	cfg    config.HeadroomStatsConfig
	client *http.Client
}

type HeadroomStatsSnapshot struct {
	Mode                   string           `json:"mode"`
	APIRequests            int64            `json:"api_requests"`
	RequestsTotal          int64            `json:"requests_total"`
	RequestsFailed         int64            `json:"requests_failed"`
	RequestsCompressed     int64            `json:"requests_compressed"`
	InputTokens            int64            `json:"input_tokens"`
	OutputTokens           int64            `json:"output_tokens"`
	TokensSaved            int64            `json:"tokens_saved"`
	ProxyCompressionSaved  int64            `json:"proxy_compression_saved"`
	TotalBeforeCompression int64            `json:"total_before_compression"`
	SavingsPercent         float64          `json:"savings_percent"`
	AverageCompressionPct  float64          `json:"average_compression_percent"`
	TotalSavedUSD          float64          `json:"total_saved_usd"`
	CostSavingsPercent     float64          `json:"cost_savings_percent"`
	ByProvider             map[string]int64 `json:"by_provider,omitempty"`
	ByModel                map[string]int64 `json:"by_model,omitempty"`
	FetchedAt              time.Time        `json:"fetched_at"`
}

func NewHeadroomStatsService(cfg *config.Config) *HeadroomStatsService {
	var statsCfg config.HeadroomStatsConfig
	if cfg != nil {
		statsCfg = cfg.HeadroomStats
	}
	timeout := time.Duration(statsCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &HeadroomStatsService{
		cfg:    statsCfg,
		client: &http.Client{Timeout: timeout},
	}
}

func (s *HeadroomStatsService) GetStats(ctx context.Context) (*HeadroomStatsSnapshot, error) {
	if s == nil || !s.cfg.Enabled {
		return nil, ErrHeadroomStatsDisabled
	}
	statsURL := strings.TrimSpace(s.cfg.URL)
	if statsURL == "" {
		return nil, fmt.Errorf("headroom stats url is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create headroom stats request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch headroom stats: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch headroom stats: status %d", resp.StatusCode)
	}

	var raw headroomStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode headroom stats: %w", err)
	}
	return raw.snapshot(time.Now().UTC()), nil
}

type headroomStatsResponse struct {
	Summary struct {
		Mode        string `json:"mode"`
		APIRequests int64  `json:"api_requests"`
		Compression struct {
			RequestsCompressed int64   `json:"requests_compressed"`
			TotalTokensRemoved int64   `json:"total_tokens_removed"`
			AvgCompressionPct  float64 `json:"avg_compression_pct"`
		} `json:"compression"`
		Cost struct {
			TotalSavedUSD float64 `json:"total_saved_usd"`
			SavingsPct    float64 `json:"savings_pct"`
		} `json:"cost"`
	} `json:"summary"`
	Requests struct {
		Total      int64            `json:"total"`
		Failed     int64            `json:"failed"`
		ByProvider map[string]int64 `json:"by_provider"`
		ByModel    map[string]int64 `json:"by_model"`
	} `json:"requests"`
	Tokens struct {
		Input                  int64   `json:"input"`
		Output                 int64   `json:"output"`
		Saved                  int64   `json:"saved"`
		ProxyCompressionSaved  int64   `json:"proxy_compression_saved"`
		TotalBeforeCompression int64   `json:"total_before_compression"`
		SavingsPercent         float64 `json:"savings_percent"`
	} `json:"tokens"`
}

func (r headroomStatsResponse) snapshot(fetchedAt time.Time) *HeadroomStatsSnapshot {
	return &HeadroomStatsSnapshot{
		Mode:                   r.Summary.Mode,
		APIRequests:            r.Summary.APIRequests,
		RequestsTotal:          r.Requests.Total,
		RequestsFailed:         r.Requests.Failed,
		RequestsCompressed:     r.Summary.Compression.RequestsCompressed,
		InputTokens:            r.Tokens.Input,
		OutputTokens:           r.Tokens.Output,
		TokensSaved:            r.Tokens.Saved,
		ProxyCompressionSaved:  r.Tokens.ProxyCompressionSaved,
		TotalBeforeCompression: r.Tokens.TotalBeforeCompression,
		SavingsPercent:         r.Tokens.SavingsPercent,
		AverageCompressionPct:  r.Summary.Compression.AvgCompressionPct,
		TotalSavedUSD:          r.Summary.Cost.TotalSavedUSD,
		CostSavingsPercent:     r.Summary.Cost.SavingsPct,
		ByProvider:             r.Requests.ByProvider,
		ByModel:                r.Requests.ByModel,
		FetchedAt:              fetchedAt,
	}
}
