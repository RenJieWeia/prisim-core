# Prism Core SDK

**Prism Core** 是 Prism 能源数据生态系统的基础 SDK。它提供了一个基于**六边形架构**的高性能、模块化数据处理引擎，专为水、电、气等异构能源数据的标准化而设计。

本库被设计为核心依赖 (Core Dependency)，供上层服务（如 HTTP API、CLI 工具、ETL 管道）引用，以提供一致的数据清洗和标准化能力。

## 🌟 核心特性

- **通用摄入 (Universal Ingestion)**: 
  - 支持基于流 (Stream) 的 **JSON** 解析，以及 **CSV** 格式支持。能够高效处理大规模数据集，内存占用极低。
- **稳健的数据清洗流水线 (Robust Pipeline)**:
  - **策略模式 (Strategy Pattern)**: 清洗规则完全解耦，通过 `RuleFactory` 支持热插拔。
  - **内置规则库**:
    - `RangeRule`: 范围检查，支持 Min/Max 阈值校验与自动修正 (Clamping)。
    - `MonotonicRule`: 单调性检查，防止累计读数回退。
    - `RateRule`: 变化率检查，检测不可能出现的尖峰跳变。
    - `StagnationRule`: 停滞检查，识别死传感器 (连续读数不变)。
    - 规则均通过 `RuleFactory` 按类型实例化 (RANGE / MONOTONIC / RATE / STAGNATION)。
  - **责任链 (Chain of Responsibility)**: 通过 `Sanitizer` 服务串行执行配置的过滤器。
  - **结构化结果**: 规则返回 `CheckResult` 结构体，包含修正状态和原因说明。
- **数据标准化 (Standardization)**:
  - **精度统一**: 将浮点数转换为高精度的整型定点数 (Scaled Integer)，默认 10000 倍精度 (4 位小数)，可通过 `WithPrecision(factor)` 配置；转换采用 **int64 截断**。
  - **时间对齐 (Aligner)**: 使用**二分查找算法** O(log n) 将散乱的时间点对齐到标准的整点快照。
- **并发安全**:
  - 异步操作带有超时控制和错误日志。
  - 使用 `errors.Join()` 聚合并发错误。
- **架构设计**:
  - **Domain (领域层)**: 核心业务实体与接口定义 (`pkg/core/domain`)。
  - **Services (服务层)**: 业务流程编排 (`pkg/core/services`)，包含 Sanitizer 与 Standardizer 实现。
  - **Adapters (适配层)**: 外部交互实现 (`pkg/adapters`)，包含 Ingestors 和 Factory。

## 🎬 快速体验 (Demo)

无需编写代码即可感受完整流水线 (接入 → 清洗 → 标准化 → 查询)：

```bash
go run ./cmd/demo              # 脚本化演示 (内置脏数据样例)
go run ./cmd/demo -ingest data.json   # 自喂数据 (.json / .csv)
```

## 📂 项目结构

```
prism-core/
├── cmd/
│   └── demo/          # 演示程序 (go run ./cmd/demo)
├── docs/
│   └── dev-guide/     # 开发手册 (六边形架构说明)
├── pkg/
│   ├── adapters/      # 适配器层 (外部交互)
│   │   ├── factory/   #   规则工厂 (RuleFactory 规则实例化)
│   │   └── ingest/    #   数据摄入实现 (CSV, JSON)
│   └── core/
│       ├── domain/    # 核心业务逻辑 (实体 & 算法)
│       │   ├── aligner.go    # 时间对齐逻辑
│       │   ├── unifier.go    # 精度转换器 (math.Round)
│       │   ├── rule.go       # 规则定义 (RuleType / RuleAction)
│       │   └── ...
│       ├── ports/     # 接口定义 (驱动/被驱动端口)
│       └── services/  # 应用服务 (流程编排)
│           ├── sanitizer.go  # 清洗器 (责任链)
│           ├── rules/        # 内置规则: Range / Monotonic / Rate / Stagnation
│           └── ...
├── tests/             # 外部集成测试 (包均为 xxx_test)
│   ├── adapters/      # 工厂与接入器测试
│   └── core/          # 领域与服务层测试
└── testdata/          # 测试用例样本数据
```

## 🚀 快速开始

### 安装

```bash
go get github.com/renjie/prism-core
```

使用 `JsonUniversalIngestor` 从文件或网络流中读取原始数据。

```go
import (
    "context"
    "fmt"
    "os"
    "github.com/renjie/prism-core/pkg/adapters/ingest"
    "github.com/renjie/prism-core/pkg/core/services"
    "github.com/renjie/prism-core/pkg/core/domain"
)

// 定义数据接收回调（模拟“下游”处理）
downstreamHandler := func(ctx context.Context, readings []domain.Reading) error {
    for _, r := range readings {
        fmt.Printf("Received: %s - %.2f\n", r.DeviceInfo.ID, r.Value)
    }
    return nil
}

// 初始化摄入器
ingestor := ingest.NewJsonUniversalIngestor(downstreamHandler)

// 打开数据源 (io.Reader)
file, _ := os.Open("data.json")
defer file.Close()

// 开始流式处理
ingestor.IngestStream(context.Background(), file)
```

### 2. 配置清洗规则与标准化服务

核心服务 `CoreStandardizer` 负责编排清洗和标准化逻辑。你可以根据业务需求注入不同的规则。

```go
import (
    "time"
    "github.com/renjie/prism-core/pkg/core/services"
    "github.com/renjie/prism-core/pkg/core/services/rules"
    "github.com/renjie/prism-core/pkg/core/domain"
)

// 使用内置的 RangeRule (范围检查)
rangeRule := &rules.RangeRule{
    Min:    0.0,
    Max:    1000.0,
    Action: domain.ActionReject, // 超出范围直接丢弃
}

// 场景：我们需要严格的数据质量控制
// 使用 Functional Options 配置服务
// 精度转换已内置，默认 ScaleFactor = 10000
standardizer := services.NewCoreStandardizer(
    services.WithCleaningRules(rangeRule),
    services.WithAlignment(15*time.Minute, 1*time.Minute),
)
```

### 3. 执行并获取结果

```go
rawReadings := []domain.Reading{
    {Timestamp: t1, Value: 100.0},
    {Timestamp: t2, Value: -5.0}, // 将被 RangeRule 过滤
    {Timestamp: t3, Value: 105.0},
}

// 执行标准化
results, err := standardizer.ProcessAndStandardize(ctx, rawReadings)

// 结果中只包含有效且转换后的数据
for _, res := range results {
    fmt.Printf("Standardized: %d (Raw: %.2f)\n", res.ValueScaled, res.ValueDisplay)
}
// Output:
// Standardized: 1000000 (Raw: 100.00)
// Standardized: 1050000 (Raw: 105.00)
```

### 4. 数据持久化 (Persistence)

`ProcessAndStandardize` 支持可选持久化：通过 `WithRepository` 注入 `ports.StandardReadingRepository` 后，处理结果会自动以 `UpsertStrategyHighPriorityWins` 策略落库。

```go
standardizer := services.NewCoreStandardizer(
    services.WithRepository(repo), // 实现 ports.StandardReadingRepository
)

// 查询特定时间点的标准读数 (需要已注入仓储)
sr, err := standardizer.GetStandardReading(ctx, "D1", t)
```

### 5. 动态规则加载 (Dynamic Rules)

按设备类型从仓储加载清洗规则时，需要同时注入规则仓储与规则工厂：

```go
import "github.com/renjie/prism-core/pkg/adapters/factory" // 具体工厂实现 (基础设施层)

standardizer := services.NewCoreStandardizer(
    services.WithRuleRepository(ruleRepo),   // 实现 ports.CleaningRuleRepository
    services.WithRuleFactory(factory.GetRuleFactory()), // ports.CleaningRuleFactory
)
```

> 依赖倒置：核心层只依赖 `ports.CleaningRuleFactory` 接口，不直接依赖 `pkg/adapters`。

## 🛠 开发与测试

本项目采用严格的测试分离策略，单元测试位于 `tests/` 目录下。

### 运行测试
确保你已安装 Go 1.25+。

```bash
# 运行所有测试
go test ./...
```

## 📄 许可证
MIT License
