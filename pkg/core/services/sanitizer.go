package services

import (
	"sort"
	"time"

	"github.com/renjie/prism-core/pkg/core/domain"
	"github.com/renjie/prism-core/pkg/core/ports"
)

// ChainSanitizer 基于责任链模式的清洗器实现
type ChainSanitizer struct {
	rules []ports.CleaningRule
}

// NewSanitizer 创建默认的基于规则链的清洗器
func NewSanitizer(rules ...ports.CleaningRule) ports.Sanitizer {
	return &ChainSanitizer{rules: rules}
}

// Clean 实现 ports.Sanitizer 接口
// 返回的 clean 数据已按时间戳升序排列
func (s *ChainSanitizer) Clean(readings []domain.Reading) ([]domain.Reading, []domain.QuarantineReading) {
	if len(readings) == 0 {
		return nil, nil
	}

	// 1. 预处理：时间排序
	sort.Slice(readings, func(i, j int) bool {
		return readings[i].Timestamp.Before(readings[j].Timestamp)
	})

	var clean []domain.Reading
	var quarantined []domain.QuarantineReading
	var prev *domain.Reading

	for _, curr := range readings {
		// 设备切换时重置上下文，避免跨设备的规则判定污染
		if prev != nil && prev.DeviceInfo.ID != curr.DeviceInfo.ID {
			prev = nil
		}

		// 0. 内置规则: 同设备下的时间戳去重
		if prev != nil && prev.DeviceInfo.ID == curr.DeviceInfo.ID && prev.Timestamp.Equal(curr.Timestamp) {
			// 重复数据视为 Dirty Data? 或者只是 Drop?
			// 策略：视为 Duplicate Error，进入 Quarantine
			q := domain.QuarantineReading{
				Reading:   curr,
				Status:    domain.QuarantineStatusPending,
				Reason:    "Duplicate timestamp",
				CreatedAt: time.Now(),
			}
			quarantined = append(quarantined, q)
			continue
		}

		// 执行规则链
		passed := true
		failReason := ""

		// 构建清洗上下文
		cleanCtx := ports.CleaningContext{
			Previous: prev,
		}

		// 每次进入规则检查时，使用当前的 curr 副本
		// 这样不同规则可以像流水线一样依次修改数据 (Pipe and Filter)
		tempReading := curr

		for _, rule := range s.rules {
			result := rule.Check(cleanCtx, tempReading)
			if !result.Passed {
				passed = false
				failReason = result.Reason
				break
			}
			// 将这一步可能修正过的结果传递给下一个规则
			tempReading = result.Reading
		}

		if passed {
			clean = append(clean, tempReading)
			// 注意：prev 指向的是已经进入 clean 列表的、可能被修正过的最终值
			prev = &clean[len(clean)-1]
		} else {
			// 只有 REJECT 的才进入这里 (CleanRule内如果自动更正则会返回ok=true)
			q := domain.QuarantineReading{
				Reading:   curr,
				Status:    domain.QuarantineStatusPending,
				Reason:    failReason,
				CreatedAt: time.Now(),
			}
			quarantined = append(quarantined, q)
		}
	}
	return clean, quarantined
}
