package services_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/renjie/prism-core/pkg/adapters/sink"
	"github.com/renjie/prism-core/pkg/core/domain"
	"github.com/renjie/prism-core/pkg/core/ports"
	"github.com/renjie/prism-core/pkg/core/services"
)

// persistRecorder 记录 SaveBatch 调用 (重复持久化检测)
type persistRecorder struct {
	mu        sync.Mutex
	saves     int
	devices   []string
	lastErr   error
	failAfter int // 前 failAfter 次成功后返回错误
}

func (r *persistRecorder) Save(_ context.Context, _ domain.StandardReading, _ ports.UpsertStrategy) error {
	return nil
}
func (r *persistRecorder) SaveBatch(_ context.Context, rs []domain.StandardReading, _ ports.UpsertStrategy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failAfter > 0 && r.saves >= r.failAfter {
		return context.DeadlineExceeded
	}
	r.saves++
	for _, s := range rs {
		r.devices = append(r.devices, s.DeviceID)
	}
	return nil
}
func (r *persistRecorder) FindExact(_ context.Context, _ string, _ time.Time) (*domain.StandardReading, error) {
	return nil, nil
}
func (r *persistRecorder) FindRange(_ context.Context, _ string, _, _ time.Time) ([]domain.StandardReading, error) {
	return nil, nil
}
func (r *persistRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saves
}

func rawReading(deviceID string, ts time.Time, v float64) domain.Reading {
	return domain.Reading{DeviceInfo: domain.DeviceInfo{ID: deviceID}, Timestamp: ts, Value: v}
}

// TestPersistenceConflictWithRepositoryAndSink WithRepository + RepositorySink 同一仓储 -> 报错
func TestPersistenceConflictWithRepositoryAndSink(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2026-08-04T10:00:00Z")
	repo := &persistRecorder{}
	std := services.NewCoreStandardizer(
		services.WithRepository(repo),
		services.WithResultSinks(sink.NewRepositorySink(repo, ports.UpsertStrategyHighPriorityWins)),
	)

	result, err := std.Process(context.Background(), []domain.Reading{
		rawReading("D1", base, 100),
	})
	if err == nil {
		t.Fatal("expected duplicate persistence error")
	}
	if !strings.Contains(err.Error(), "duplicate persistence target") {
		t.Errorf("expected duplicate persistence error, got: %v", err)
	}
	// fail-fast: 不应执行任何 SaveBatch
	if repo.count() != 0 {
		t.Errorf("expected no SaveBatch before conflict error, got %d", repo.count())
	}
	_ = result
}

// TestPersistenceConflictTwoRepositorySinks 两个 RepositorySink 指向同一仓储 -> 报错
func TestPersistenceConflictTwoRepositorySinks(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2026-08-04T10:00:00Z")
	repo := &persistRecorder{}
	std := services.NewCoreStandardizer(
		services.WithResultSinks(
			sink.NewRepositorySink(repo, ports.UpsertStrategyHighPriorityWins),
			sink.NewRepositorySink(repo, ports.UpsertStrategyLastWriteWins),
		),
	)

	_, err := std.Process(context.Background(), []domain.Reading{
		rawReading("D1", base, 100),
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate persistence target") {
		t.Fatalf("expected duplicate persistence error, got: %v", err)
	}
	if repo.count() != 0 {
		t.Errorf("expected no SaveBatch before conflict error, got %d", repo.count())
	}
}

// TestWithRepositoryPersistsOnce WithRepository 单独使用 -> 恰好持久化一次
func TestWithRepositoryPersistsOnce(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2026-08-04T10:00:00Z")
	repo := &persistRecorder{}
	std := services.NewCoreStandardizer(services.WithRepository(repo))

	result, err := std.Process(context.Background(), []domain.Reading{
		rawReading("D1", base, 100),
		rawReading("D1", base.Add(15*time.Minute), 110),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.count() != 1 {
		t.Fatalf("expected exactly 1 SaveBatch, got %d", repo.count())
	}
	if len(result.Accepted) != len(repo.devices) {
		t.Errorf("expected all accepted persisted, accepted=%d saved=%d", len(result.Accepted), len(repo.devices))
	}
}

// TestRepositorySinkPersistsOnce RepositorySink 单独使用 -> 恰好持久化一次
func TestRepositorySinkPersistsOnce(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2026-08-04T10:00:00Z")
	repo := &persistRecorder{}
	std := services.NewCoreStandardizer(
		services.WithResultSinks(sink.NewRepositorySink(repo, ports.UpsertStrategyHighPriorityWins)),
	)

	_, err := std.Process(context.Background(), []domain.Reading{
		rawReading("D1", base, 100),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.count() != 1 {
		t.Fatalf("expected exactly 1 SaveBatch, got %d", repo.count())
	}
}

var _ ports.StandardReadingRepository = (*persistRecorder)(nil)
