package domain

// RuleOutcome 规则执行结果类型
type RuleOutcome string

const (
	// RuleOutcomePass 规则通过
	RuleOutcomePass RuleOutcome = "PASS"
	// RuleOutcomeReject 规则拒绝
	RuleOutcomeReject RuleOutcome = "REJECT"
	// RuleOutcomeSkip 规则被跳过 (如缺少参考对象)
	RuleOutcomeSkip RuleOutcome = "SKIP"
	// RuleOutcomeCorrect 规则通过并进行了修正
	RuleOutcomeCorrect RuleOutcome = "CORRECT"
	// RuleOutcomeQuarantine 数据被隔离审查
	RuleOutcomeQuarantine RuleOutcome = "QUARANTINE"
)

// RuleEvaluation 记录一条规则对一条读数的评估结果
type RuleEvaluation struct {
	RuleID       string   // 执行了哪条规则
	Outcome      string   // PASS / REJECT / SKIP / CORRECT / QUARANTINE
	Reason       string   // 失败或跳过原因
	ReferenceIDs []string // 使用了哪些参考对象 (参考规则的 ReferenceSpec.ID)
}

// ProcessingResult 完整的处理结果
type ProcessingResult struct {
	Accepted    []StandardReading   // 通过清洗并标准化的数据
	Rejected    []QuarantineReading // 被拒绝/隔离的数据
	Evaluations []RuleEvaluation    // 每条规则对每条读数的评估记录
}
