package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/renjie/prism-core/pkg/adapters/factory"
	"github.com/renjie/prism-core/pkg/core/domain"
	"github.com/renjie/prism-core/pkg/core/ports"
	"github.com/renjie/prism-core/pkg/core/services"
)

// fakeRuleRepo 内存版规则仓储 (测试用)
type fakeRuleRepo struct {
	rules map[domain.DeviceType][]domain.CleaningRule
}

func (f *fakeRuleRepo) Save(_ context.Context, rule domain.CleaningRule) error { return nil }
func (f *fakeRuleRepo) GetByID(_ context.Context, id string) (*domain.CleaningRule, error) {
	return nil, nil
}
func (f *fakeRuleRepo) ListByDeviceType(_ context.Context, dt domain.DeviceType) ([]domain.CleaningRule, error) {
	return f.rules[dt], nil
}
func (f *fakeRuleRepo) ListEnabledByDeviceType(_ context.Context, dt domain.DeviceType) ([]domain.CleaningRule, error) {
	var out []domain.CleaningRule
	for _, r := range f.rules[dt] {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRuleRepo) Delete(_ context.Context, id string) error { return nil }

// quarantineRecorder 记录异步保存的隔离区数据 (测试用)
type quarantineRecorder struct {
	mu    sync.Mutex
	saves []domain.QuarantineReading
}

func (q *quarantineRecorder) Save(_ context.Context, record domain.QuarantineReading) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.saves = append(q.saves, record)
	return nil
}
func (q *quarantineRecorder) FindPending(_ context.Context, limit int) ([]domain.QuarantineReading, error) {
	return nil, nil
}
func (q *quarantineRecorder) count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.saves)
}

func TestProcessWithDynamicRules(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2023-01-01T10:00:00Z")
	ruleRepo := &fakeRuleRepo{rules: map[domain.DeviceType][]domain.CleaningRule{
		domain.DeviceTypeElec: {
			{ID: "r1", Type: domain.RuleTypeRange, Enabled: true, Parameters: map[string]interface{}{"min": float64(0), "max": float64(1000)}},
		},
	}}
	quarantine := &quarantineRecorder{}

	standardizer := services.NewCoreStandardizer(
		services.WithRuleRepository(ruleRepo),
		services.WithRuleFactory(factory.NewRuleFactory()),
		services.WithQuarantineRepository(quarantine),
	)

	readings := []domain.Reading{
		{DeviceInfo: domain.DeviceInfo{ID: "D1", Type: domain.DeviceTypeElec}, Timestamp: base, Value: 100},
		{DeviceInfo: domain.DeviceInfo{ID: "D2", Type: domain.DeviceTypeElec}, Timestamp: base.Add(time.Minute), Value: -5}, // 越界
	}

	results, err := standardizer.ProcessAndStandardize(context.Background(), readings)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 clean reading, got %d", len(results))
	}

	// 隔离区保存是异步的，轮询等待
	deadline := time.Now().Add(2 * time.Second)
	for quarantine.count() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if quarantine.count() != 1 {
		t.Fatalf("expected 1 quarantined reading saved, got %d", quarantine.count())
	}
}

func TestDynamicCleaningRequiresFactory(t *testing.T) {
	// 配置了规则仓储但未注入规则工厂 -> 应报错 (依赖倒置保护)
	ruleRepo := &fakeRuleRepo{rules: map[domain.DeviceType][]domain.CleaningRule{}}
	standardizer := services.NewCoreStandardizer(
		services.WithRuleRepository(ruleRepo),
	)
	base, _ := time.Parse(time.RFC3339, "2023-01-01T10:00:00Z")
	readings := []domain.Reading{
		{DeviceInfo: domain.DeviceInfo{ID: "D1", Type: domain.DeviceTypeElec}, Timestamp: base, Value: 100},
	}
	if _, err := standardizer.ProcessAndStandardize(context.Background(), readings); err == nil {
		t.Fatalf("expected error when rule factory is missing")
	}
}

var _ ports.CleaningRuleRepository = (*fakeRuleRepo)(nil)
var _ ports.QuarantineRepository = (*quarantineRecorder)(nil)
