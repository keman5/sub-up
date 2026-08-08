package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpsHostHealthServiceReadsSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host-health.json")
	now := time.Now().UTC().Format(time.RFC3339)
	require.NoError(t, os.WriteFile(path, []byte(`{
		"collected_at":"`+now+`",
		"load_average":{"one":2.1,"five":1.8,"fifteen":1.4},
		"cpu":{"usage_percent":96.5,"high":true},
		"memory":{"available_mb":1720,"swap_used_mb":177},
		"top_containers":[{"name":"sub2api-worker","cpu_percent":163.5,"memory":"936MiB / 1.172GiB","pids":21}],
		"top_processes":[{"pid":1234,"command":"python","cpu_percent":160.2,"rss_mb":936}],
		"diagnosis":"CPU 压力主要来自 sub2api-worker"
	}`), 0o644))

	svc := NewOpsHostHealthService(path)
	snapshot, err := svc.GetSnapshot(context.Background())

	require.NoError(t, err)
	require.True(t, snapshot.Available)
	require.False(t, snapshot.Stale)
	require.InDelta(t, 2.1, snapshot.LoadAverage.One, 0.001)
	require.InDelta(t, 96.5, snapshot.CPU.UsagePercent, 0.001)
	require.True(t, snapshot.CPU.High)
	require.Equal(t, int64(1720), snapshot.Memory.AvailableMB)
	require.Len(t, snapshot.TopContainers, 1)
	require.Equal(t, "sub2api-worker", snapshot.TopContainers[0].Name)
	require.Len(t, snapshot.TopProcesses, 1)
	require.Equal(t, int64(1234), snapshot.TopProcesses[0].PID)
	require.Equal(t, "CPU 压力主要来自 sub2api-worker", snapshot.Diagnosis)
}

func TestOpsHostHealthServiceMarksStaleSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host-health.json")
	old := time.Now().UTC().Add(-3 * time.Minute).Format(time.RFC3339)
	require.NoError(t, os.WriteFile(path, []byte(`{"collected_at":"`+old+`","cpu":{"usage_percent":12.3}}`), 0o644))

	svc := NewOpsHostHealthService(path)
	snapshot, err := svc.GetSnapshot(context.Background())

	require.NoError(t, err)
	require.True(t, snapshot.Available)
	require.True(t, snapshot.Stale)
	require.Equal(t, "stale", snapshot.Status)
}

func TestOpsHostHealthServiceReturnsUnavailableWhenSnapshotMissing(t *testing.T) {
	svc := NewOpsHostHealthService(filepath.Join(t.TempDir(), "missing.json"))

	snapshot, err := svc.GetSnapshot(context.Background())

	require.NoError(t, err)
	require.False(t, snapshot.Available)
	require.Equal(t, "missing", snapshot.Status)
	require.Equal(t, "host health snapshot is not available", snapshot.Message)
}
