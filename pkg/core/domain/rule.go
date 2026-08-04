package domain

// RuleType 定义清洗规则类型
type RuleType string

const (
	RuleTypeRange      RuleType = "RANGE"      // 范围检查 (Min/Max)
	RuleTypeRate       RuleType = "RATE"       // 变化率检查 (尖峰/跳变检测)
	RuleTypeTrend      RuleType = "TREND"      // 趋势检查 (预留)
	RuleTypeMonotonic  RuleType = "MONOTONIC"  // 单调性检查 (防止累计值回退)
	RuleTypeStagnation RuleType = "STAGNATION" // 停滞检查 (死传感器检测)
)

// RuleAction 定义规则触发后的处理策略
type RuleAction string

const (
	ActionReject   RuleAction = "REJECT"    // 默认：丢弃数据 (Error)
	ActionCorrect  RuleAction = "CORRECT"   // 修正：修改值并标记为 CORRECTED
	ActionFlagOnly RuleAction = "FLAG_ONLY" // 仅标记：保留原值，但标记 Configurable Quality (暂未实现完全逻辑，可作为 VALID 但带警告)
)

// CleaningRule 定义数据清洗规则
// 这些规则用于 Standardizer 服务中，决定哪些原始数据是异常的
type CleaningRule struct {
	ID         string         `json:"id"`
	DeviceType DeviceType     `json:"device_type"` // 规则适用的设备类型
	Type       RuleType       `json:"type"`
	Action     RuleAction     `json:"action"` // 触发规则后的行为 (REJECT / CORRECT)
	Enabled    bool           `json:"enabled"`
	Parameters map[string]any `json:"parameters"` // 规则参数 (例如: {"min": 0, "max": 100})
	Priority   int            `json:"priority"`   // 执行优先级
}
