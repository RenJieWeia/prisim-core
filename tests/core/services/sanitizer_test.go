package services_test

import (
	"testing"
	"time"

	"github.com/renjie/prism-core/pkg/core/domain"
	"github.com/renjie/prism-core/pkg/core/services"
	"github.com/renjie/prism-core/pkg/core/services/rules"
)

func TestChainSanitizerClean(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2023-01-01T10:00:00Z")
	sanitizer := services.NewSanitizer(&rules.RangeRule{Min: 0, Max: 1000})

	readings := []domain.Reading{
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base.Add(30 * time.Minute), Value: 110},
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base, Value: 100},
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base, Value: 100},                      // 重复时间戳
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base.Add(15 * time.Minute), Value: -5}, // 越界
	}

	clean, quarantined := sanitizer.Clean(readings)

	// 排序升序 + 去重 + 规则过滤后应为 [100, 110]
	if len(clean) != 2 {
		t.Fatalf("expected 2 clean readings, got %d", len(clean))
	}
	if clean[0].Value != 100 || clean[1].Value != 110 {
		t.Errorf("unexpected clean values: %v, %v", clean[0].Value, clean[1].Value)
	}
	if !clean[0].Timestamp.Before(clean[1].Timestamp) {
		t.Errorf("clean readings not sorted ascending")
	}

	// 隔离区应有 2 条: 重复 + 越界
	if len(quarantined) != 2 {
		t.Fatalf("expected 2 quarantined readings, got %d", len(quarantined))
	}
}

func TestChainSanitizerCorrects(t *testing.T) {
	base, _ := time.Parse(time.RFC3339, "2023-01-01T10:00:00Z")
	// CORRECT 模式下越界值被钳制而非丢弃
	sanitizer := services.NewSanitizer(&rules.RangeRule{Min: 0, Max: 100, Action: domain.ActionCorrect})

	readings := []domain.Reading{
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base, Value: 50},
		{DeviceInfo: domain.DeviceInfo{ID: "D1"}, Timestamp: base.Add(15 * time.Minute), Value: 150}, // 越界 -> 钳制到 100
	}

	clean, quarantined := sanitizer.Clean(readings)
	if len(clean) != 2 {
		t.Fatalf("expected 2 clean readings after correction, got %d", len(clean))
	}
	if clean[1].Value != 100 {
		t.Errorf("expected corrected value 100, got %v", clean[1].Value)
	}
	if len(quarantined) != 0 {
		t.Errorf("expected no quarantined readings, got %d", len(quarantined))
	}
}

func TestChainSanitizerEmpty(t *testing.T) {
	sanitizer := services.NewSanitizer()
	clean, quarantined := sanitizer.Clean(nil)
	if clean != nil || quarantined != nil {
		t.Errorf("expected nil results for empty input, got %v / %v", clean, quarantined)
	}
}
