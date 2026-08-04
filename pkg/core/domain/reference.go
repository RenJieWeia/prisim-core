package domain

import "time"

// ReferenceSourceKind 定义参考对象的数据来源
type ReferenceSourceKind string

const (
	// ReferenceSourceCurrentBatch 从当前输入批次中查询参考数据
	ReferenceSourceCurrentBatch ReferenceSourceKind = "CURRENT_BATCH"

	// ReferenceSourceStandardRepo 从标准数据仓储中查询历史参考数据
	ReferenceSourceStandardRepo ReferenceSourceKind = "STANDARD_REPO"
)

// ReferenceBinding 定义参考对象与设备的绑定方式
type ReferenceBinding string

const (
	// ReferenceBindingSameDevice 使用当前读数同一设备的数据
	ReferenceBindingSameDevice ReferenceBinding = "SAME_DEVICE"

	// ReferenceBindingExplicit 使用显式指定的设备数据
	ReferenceBindingExplicit ReferenceBinding = "EXPLICIT"
)

// ReferenceTimeMode 定义参考对象的时间选择方式
type ReferenceTimeMode string

const (
	// ReferenceTimePrevious 当前序列上一条有效数据
	ReferenceTimePrevious ReferenceTimeMode = "PREVIOUS"

	// ReferenceTimeRelative 相对当前数据时间偏移的某个时间点
	ReferenceTimeRelative ReferenceTimeMode = "RELATIVE"

	// ReferenceTimeWindow 以当前数据时间往前追溯的一个时间窗口
	ReferenceTimeWindow ReferenceTimeMode = "WINDOW"
)

// ReferenceTimeSelector 时间选择器
// Offset 含义随 Mode 变化:
//   - RELATIVE: 相对当前数据时间往回偏移的时长 (目标时间 = 当前时间 - Offset)
//   - WINDOW:   窗口长度 (窗口 = [当前时间-Offset, 当前时间))
//
// Tolerance 仅 RELATIVE 模式使用: 在 [目标时间-Tolerance, 目标时间+Tolerance] 内查找。
// 若 Tolerance <= 0，则要求目标时间点上存在精确数据。
type ReferenceTimeSelector struct {
	Mode      ReferenceTimeMode
	Offset    time.Duration
	Tolerance time.Duration
}

// ReferenceReducer 定义时间窗口内多条数据的聚合方式
type ReferenceReducer string

const (
	ReferenceReducerAvg    ReferenceReducer = "AVG"    // 平均值
	ReferenceReducerSum    ReferenceReducer = "SUM"    // 求和
	ReferenceReducerMin    ReferenceReducer = "MIN"    // 最小值
	ReferenceReducerMax    ReferenceReducer = "MAX"    // 最大值
	ReferenceReducerLatest ReferenceReducer = "LATEST" // 最近一条 (按时间最新)
	ReferenceReducerDelta  ReferenceReducer = "DELTA"  // 首尾差值 (最后一条 - 第一条)
)

// MissingReferencePolicy 定义找不到参考数据时的处理策略
type MissingReferencePolicy string

const (
	// MissingReferenceSkip 跳过该规则 (当前数据继续走剩余规则链)
	MissingReferenceSkip MissingReferencePolicy = "SKIP_RULE"

	// MissingReferenceReject 拒绝当前数据 (进入隔离区)
	MissingReferenceReject MissingReferencePolicy = "REJECT"

	// MissingReferenceQuarantine 隔离当前数据 (进入隔离区等待人工审查)
	MissingReferenceQuarantine MissingReferencePolicy = "QUARANTINE"
)

// ReferenceSpec 参考对象定义
// 声明"当前数据与什么数据比较": 数据来源、设备绑定、时间选择、聚合方式与缺失策略。
type ReferenceSpec struct {
	ID            string
	Source        ReferenceSourceKind
	Binding       ReferenceBinding
	DeviceID      string // 仅 Binding == EXPLICIT 时使用
	Time          ReferenceTimeSelector
	Reducer       ReferenceReducer
	MissingPolicy MissingReferencePolicy
}

// ReferenceRequest 一次具体的参考数据查询请求
// 由 ReferenceSpec + 当前读数解析而来，提交给 ReferenceSource 查询。
type ReferenceRequest struct {
	ID       string
	Source   ReferenceSourceKind
	DeviceID string
	Mode     ReferenceTimeMode

	// Target 精确目标时间点 (PREVIOUS: 当前读数时间; RELATIVE: 偏移后的目标时间)
	Target time.Time

	// Start / End 时间窗口边界 [Start, End) (WINDOW 模式)
	Start time.Time
	End   time.Time

	// Tolerance RELATIVE 模式查找容差 ([Target-Tolerance, Target+Tolerance])
	Tolerance time.Duration

	Reducer ReferenceReducer
}

// ReferencePoint 参考窗口内的一条数据点
type ReferencePoint struct {
	Timestamp time.Time
	Value     float64
}

// ReferenceValue 参考数据查询结果
// 单点查询 (PREVIOUS/RELATIVE) 使用 Value; 窗口查询 (WINDOW) 使用聚合后的 Value 与原始 Points。
type ReferenceValue struct {
	Found     bool
	Timestamp time.Time
	Value     float64
	Points    []ReferencePoint
}

// ReducePoints 对时间窗口内的数据点执行聚合
// points 必须按 Timestamp 升序排列。
// 返回 (聚合值, 是否成功)。空集返回 (0, false)。
func ReducePoints(points []ReferencePoint, reducer ReferenceReducer) (float64, bool) {
	if len(points) == 0 {
		return 0, false
	}

	switch reducer {
	case ReferenceReducerSum:
		var sum float64
		for _, p := range points {
			sum += p.Value
		}
		return sum, true

	case ReferenceReducerAvg:
		var sum float64
		for _, p := range points {
			sum += p.Value
		}
		return sum / float64(len(points)), true

	case ReferenceReducerMin:
		min := points[0].Value
		for _, p := range points {
			if p.Value < min {
				min = p.Value
			}
		}
		return min, true

	case ReferenceReducerMax:
		max := points[0].Value
		for _, p := range points {
			if p.Value > max {
				max = p.Value
			}
		}
		return max, true

	case ReferenceReducerLatest:
		return points[len(points)-1].Value, true

	case ReferenceReducerDelta:
		return points[len(points)-1].Value - points[0].Value, true

	default:
		// 未知聚合方式视为无效
		return 0, false
	}
}
