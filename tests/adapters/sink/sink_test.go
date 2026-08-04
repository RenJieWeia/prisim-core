package sink_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/renjie/prism-core/pkg/adapters/sink"
	"github.com/renjie/prism-core/pkg/core/domain"
	"github.com/renjie/prism-core/pkg/core/ports"
	"github.com/renjie/prism-core/pkg/core/services"
)

func sampleResult() domain.ProcessingResult {
	return domain.ProcessingResult{
		Accepted: []domain.StandardReading{
			{DeviceID: "D1", ValueScaled: 1000000},
			{DeviceID: "D2", ValueScaled: 500000},
		},
		Rejected: []domain.QuarantineReading{
			{Reading: domain.Reading{DeviceInfo: domain.DeviceInfo{ID: "D3"}}, Reason: "bad"},
		},
		Evaluations: []domain.RuleEvaluation{
			{RuleID: "r1", Outcome: "PASS"},
		},
	}
}

// TestMemorySinkReceivesResult MemorySink 能收到结果
func TestMemorySinkReceivesResult(t *testing.T) {
	ms := sink.NewMemorySink()
	if ms.ID() == "" {
		t.Error("expected non-empty ID")
	}

	result := sampleResult()
	if err := ms.Deliver(context.Background(), result); err != nil {
		t.Fatalf("deliver failed: %v", err)
	}

	got := ms.Results()
	if ms.Count() != 1 {
		t.Fatalf("expected 1 result, got %d", ms.Count())
	}
	if len(got[0].Accepted) != 2 || len(got[0].Rejected) != 1 || len(got[0].Evaluations) != 1 {
		t.Errorf("unexpected stored result: %+v", got[0])
	}
}

// TestCallbackSinkReceivesResult CallbackSink 能收到结果
func TestCallbackSinkReceivesResult(t *testing.T) {
	var mu sync.Mutex
	var got *domain.ProcessingResult

	cb := sink.NewCallbackSink("cb", func(_ context.Context, result domain.ProcessingResult) error {
		cp := result
		mu.Lock()
		defer mu.Unlock()
		got = &cp
		return nil
	})

	if err := cb.Deliver(context.Background(), sampleResult()); err != nil {
		t.Fatalf("deliver failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if got == nil {
		t.Fatal("expected callback to receive result")
	}
	if len(got.Accepted) != 2 {
		t.Errorf("expected 2 accepted, got %d", len(got.Accepted))
	}
}

// recordingRepo 记录 SaveBatch 保存的标准数据
type recordingRepo struct {
	mu     sync.Mutex
	saved  []domain.StandardReading
	strats []ports.UpsertStrategy
}

func (r *recordingRepo) Save(_ context.Context, _ domain.StandardReading, _ ports.UpsertStrategy) error {
	return nil
}
func (r *recordingRepo) SaveBatch(_ context.Context, rs []domain.StandardReading, strategy ports.UpsertStrategy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = append(r.saved, rs...)
	r.strats = append(r.strats, strategy)
	return nil
}
func (r *recordingRepo) FindExact(_ context.Context, _ string, _ time.Time) (*domain.StandardReading, error) {
	return nil, nil
}
func (r *recordingRepo) FindRange(_ context.Context, _ string, _, _ time.Time) ([]domain.StandardReading, error) {
	return nil, nil
}

// TestRepositorySinkOnlySavesAccepted RepositorySink 只保存 Accepted 数据
func TestRepositorySinkOnlySavesAccepted(t *testing.T) {
	repo := &recordingRepo{}
	rs := sink.NewRepositorySink(repo, ports.UpsertStrategyHighPriorityWins)

	result := sampleResult()
	if err := rs.Deliver(context.Background(), result); err != nil {
		t.Fatalf("deliver failed: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saved) != 2 {
		t.Fatalf("expected 2 saved accepted readings, got %d", len(repo.saved))
	}
	if len(repo.strats) != 1 || repo.strats[0] != ports.UpsertStrategyHighPriorityWins {
		t.Errorf("expected HighPriorityWins strategy, got %v", repo.strats)
	}
}

// TestRepositorySinkEmptyAccepted 空 Accepted 不应调用仓储
func TestRepositorySinkEmptyAccepted(t *testing.T) {
	repo := &recordingRepo{}
	rs := sink.NewRepositorySink(repo, ports.UpsertStrategyLastWriteWins)

	if err := rs.Deliver(context.Background(), domain.ProcessingResult{}); err != nil {
		t.Fatalf("deliver failed: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saved) != 0 {
		t.Errorf("expected no saves for empty accepted, got %d", len(repo.saved))
	}
}

// TestCoreStandardizerDeliversToSinks 集成: CoreStandardizer.Process 投递到多个 Sink
func TestCoreStandardizerDeliversToSinks(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2026-08-04T10:00:00Z")
	ms := sink.NewMemorySink()
	var cbGot *domain.ProcessingResult
	cb := sink.NewCallbackSink("cb", func(_ context.Context, result domain.ProcessingResult) error {
		cp := result
		cbGot = &cp
		return nil
	})

	std := services.NewEnergyDataProcessor(services.WithResultSinks(ms, cb))
	raw := []domain.Reading{
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base, Value: 100},
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base.Add(15 * time.Minute), Value: 90},
	}
	result, err := std.Process(context.Background(), raw)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if ms.Count() != 1 {
		t.Fatalf("expected MemorySink to receive 1 result, got %d", ms.Count())
	}
	if cbGot == nil {
		t.Fatal("expected CallbackSink to receive result")
	}
	if len(result.Accepted) != len(ms.Results()[0].Accepted) {
		t.Errorf("unexpected delivered result")
	}
}

// TestSinkErrorPropagates Sink 错误能够正确返回 (含 Sink ID 与原始错误, 且返回已生成的 ProcessingResult)
func TestSinkErrorPropagates(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2026-08-04T10:00:00Z")
	bad := sink.NewCallbackSink("bad", func(_ context.Context, _ domain.ProcessingResult) error {
		return errors.New("boom")
	})

	std := services.NewEnergyDataProcessor(services.WithResultSinks(bad))
	raw := []domain.Reading{
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base, Value: 100},
	}
	result, err := std.Process(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error from sink to propagate")
	}
	// 错误必须包含 Sink ID 与原始错误
	if !strings.Contains(err.Error(), `sink "bad"`) {
		t.Errorf("expected error to contain sink ID 'bad', got %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error to contain 'boom', got %v", err)
	}
	// 即使 Sink 投递失败，也返回已经生成的 ProcessingResult
	if len(result.Accepted) != 1 {
		t.Errorf("expected ProcessingResult still returned with accepted data, got %d", len(result.Accepted))
	}
}

// TestSinkPartialFailureDeliversAll 多 Sink 部分失败: 其余 Sink 仍收到结果, 错误聚合所有失败
func TestSinkPartialFailureDeliversAll(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2026-08-04T10:00:00Z")
	good := sink.NewMemorySink()
	bad1 := sink.NewCallbackSink("bad-1", func(_ context.Context, _ domain.ProcessingResult) error {
		return errors.New("first boom")
	})
	bad2 := sink.NewCallbackSink("bad-2", func(_ context.Context, _ domain.ProcessingResult) error {
		return errors.New("second boom")
	})

	std := services.NewEnergyDataProcessor(services.WithResultSinks(good, bad1, bad2))
	raw := []domain.Reading{
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base, Value: 100},
	}
	result, err := std.Process(context.Background(), raw)
	if err == nil {
		t.Fatal("expected aggregated sink errors")
	}
	// 两个失败 Sink 的错误都包含其 ID 与原始错误
	for _, want := range []string{`sink "bad-1"`, "first boom", `sink "bad-2"`, "second boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to contain %q, got: %v", want, err)
		}
	}
	// 其余 Sink 仍然收到结果 (非原子投递)
	if good.Count() != 1 {
		t.Errorf("expected good sink to still receive result, got %d", good.Count())
	}
	// 返回已经生成的 ProcessingResult
	if len(result.Accepted) != 1 {
		t.Errorf("expected ProcessingResult still returned, got %d accepted", len(result.Accepted))
	}
}

var _ ports.StandardReadingRepository = (*recordingRepo)(nil)
