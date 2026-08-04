package ports

import (
	"context"

	"github.com/renjie/prism-core/pkg/core/domain"
)

// ResultSink 处理结果输出端口
// 将完整的处理结果交付给下游，由适配器层实现具体的投递方式。
type ResultSink interface {
	// ID 输出端唯一标识
	ID() string

	// Deliver 交付一次处理结果
	Deliver(
		ctx context.Context,
		result domain.ProcessingResult,
	) error
}

// RepositoryAwareSink 暴露仓储的输出端口
// 用于检测 WithRepository 与 RepositorySink 指向同一仓储时的重复持久化。
type RepositoryAwareSink interface {
	ResultSink

	// Repository 返回该输出端口持久化所用的标准读数仓储
	Repository() StandardReadingRepository
}
