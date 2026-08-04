package domain

// RuleInput 参考规则执行时的完整输入上下文
type RuleInput struct {
	Current    Reading
	Previous   *Reading
	References map[string]ReferenceValue
}
