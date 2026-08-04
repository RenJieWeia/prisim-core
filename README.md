# Prism Core SDK

[中文文档 (Chinese Documentation)](README_CN.md)

**Prism Core** is the foundational SDK for the Prism energy data ecosystem. It provides a highly modular, hexagonally architected engine for standardizing energy and utilities data (Water, Electricity, Gas) from heterogeneous sources.

This library is designed to be imported by other services (HTTP APIs, CLI tools, ETL pipelines) to provide consistent data processing capabilities.

## 🌟 Core Features

- **Universal Ingestion**: Stream-based JSON ingestor capable of handling large datasets efficiently with minimal memory footprint.
- **Robust Cleaning Pipeline**:
  - **Strategy Pattern** based cleaning rules.
  - **Pluggable Rules**:
    - `MonotonicRule`: Prevents negative accumulation/regressions.
    - `RateRule`: Detects and filters impossible spikes (jumps).
    - `StagnationRule`: Identifies dead sensors.
    - `RangeRule`: Min/Max threshold checks with optional clamping.
    - Rules are instantiated from config via `RuleFactory` (RANGE / MONOTONIC / RATE / STAGNATION).
  - **Chain of Responsibility**: `Sanitizer` runs a configurable chain of filters.
- **Data Standardization**:
  - **Precision Control**: floating-point readings → high-precision `int64` scaled values (default ×10000), configurable via `WithPrecision(factor)`. Conversion uses **int64 truncation**.
  - **Time Alignment**: `Aligner` (O(log n) binary search) snaps readings to standard intervals (Snapshots) within a configurable tolerance.
- **Hexagonal Architecture**:
  - **Domain**: Pure business logic & entities (`pkg/core/domain`) — `Reading`, `StandardReading`, `Aligner`, `IngestContext`.
  - **Ports**: Interface definitions (`pkg/core/ports`) — `CleaningRule`, `Sanitizer`, `EnergyDataStandardizer`, repositories, `CleaningRuleFactory`.
  - **Services**: Orchestration layer (`pkg/core/services`) — `CoreStandardizer`, `ChainSanitizer`, built-in rules.
  - **Adapters**: Infrastructure (`pkg/adapters`) — JSON/CSV ingestors, `RuleFactory`.

## 🎬 快速体验 (Demo)

无需编写代码即可感受完整流水线 (接入 → 清洗 → 标准化 → 查询)：

```bash
go run ./cmd/demo              # 脚本化演示 (内置脏数据样例)
go run ./cmd/demo -ingest data.json   # 自喂数据 (.json / .csv)
```

## 📂 Project Structure

```
prism-core/
├── cmd/
│   └── demo/           # Demo program (go run ./cmd/demo)
├── docs/
│   └── dev-guide/      # Developer handbook (hexagonal architecture)
├── pkg/
│   ├── adapters/       # Infrastructure layer
│   │   ├── factory/    #   RuleFactory (rule instantiation)
│   │   └── ingest/     #   JSON / CSV ingestors
│   └── core/
│       ├── domain/     # Pure business logic (entities & algorithms)
│       ├── ports/      # Interface definitions (driver / driven ports)
│       └── services/   # Application services (orchestration + built-in rules)
│           └── rules/  #   Range / Monotonic / Rate / Stagnation
├── tests/              # External tests (packages are xxx_test)
└── testdata/           # Test sample data
```

## 🚀 Getting Started

### Installation

```bash
go get github.com/renjie/prism-core
```

### Usage Example

```go
package main

import (
    "context"
    "fmt"
    "strings"
    "time"

    // Import from the public package path
    "github.com/renjie/prism-core/pkg/adapters/ingest"
    "github.com/renjie/prism-core/pkg/core/domain"
    "github.com/renjie/prism-core/pkg/core/services"
)

func main() {
    // 1. Setup Standardization Service
    // 15-minute alignment, 4-decimal precision (x10000)
    standardizer := services.NewCoreStandardizer(
        services.WithAlignment(15*time.Minute, 5*time.Minute),
        services.WithPrecision(10000),
    )

    // 2. Process raw readings -> standardized grid output
    ctx := context.Background()
    raw := []domain.Reading{
        {DeviceInfo: domain.DeviceInfo{ID: "D1", Type: domain.DeviceTypeElec},
         Timestamp: time.Now().Truncate(15 * time.Minute), Value: 100.0},
        {DeviceInfo: domain.DeviceInfo{ID: "D1", Type: domain.DeviceTypeElec},
         Timestamp: time.Now().Truncate(15 * time.Minute).Add(15 * time.Minute), Value: 105.0},
    }
    results, err := standardizer.ProcessAndStandardize(ctx, raw)
    if err != nil {
        panic(err)
    }
    for _, r := range results {
        fmt.Printf("%s ValueScaled=%d x%d\n", r.DeviceID, r.ValueScaled, r.ScaleFactor)
    }

    // 3. Or ingest from a stream (JSON array/object) via the universal ingestor
    ing := ingest.NewJsonUniversalIngestor(func(ctx context.Context, rs []domain.Reading) error {
        _, err := standardizer.ProcessAndStandardize(ctx, rs)
        return err
    })
    ing.IngestStream(ctx, strings.NewReader(`[{"device_id":"D1","timestamp":"2026-01-01T10:00:00Z","value":100}]`))
}
```

### Running Tests

```bash
go test ./...
```

## 🛠 Architecture

### domain.Sanitizer
The `Standardizer` service cleans incoming raw data using a chain of injected rules.

```go
// Define custom rules implementing ports.CleaningRule
type MaxLimitRule struct{ limit float64 }

func (r *MaxLimitRule) Check(ctx ports.CleaningContext, curr domain.Reading) ports.CheckResult {
    if curr.Value > r.limit {
        return ports.CheckResult{
            Reading: curr,
            Passed:  false,
            Reason:  fmt.Sprintf("value %.2f exceeded limit %.2f", curr.Value, r.limit),
        }
    }
    return ports.CheckResult{Reading: curr, Passed: true}
}

// Injected via functional options
svc := services.NewCoreStandardizer(services.WithCleaningRules(&MaxLimitRule{100}))
```

For **dynamic rule loading** (rules configured per device type in a repository), you must inject both a rule repository and a rule factory:

```go
import "github.com/renjie/prism-core/pkg/adapters/factory" // concrete factory (infrastructure layer)

svc := services.NewCoreStandardizer(
    services.WithRuleRepository(ruleRepo),   // ports.CleaningRuleRepository
    services.WithRuleFactory(factory.GetRuleFactory()), // ports.CleaningRuleFactory
)
```
The `CoreStandardizer` core layer depends only on the `ports.CleaningRuleFactory` interface, never on `pkg/adapters` directly.

### Precision Conversion
The `CoreStandardizer` handles the conversion between "Human Readable" floats and "Machine Precise" integers automatically.

```go
// Default: 4 decimal places precision (x10000)
// 100.00019 * 10000 = 1000001 (int64 truncation)
standardizer := services.NewCoreStandardizer()
results, _ := standardizer.ProcessAndStandardize(ctx, readings)
// results[0].ValueScaled = 1000001
// results[0].ScaleFactor = 10000
// results[0].ValueDisplay = 100.00019 (original preserved)
```

### Aligner (Time Alignment)
Uses **binary search** (O(log n)) to find the nearest reading within tolerance:

```go
aligner := domain.NewAligner(5 * time.Minute) // 5-minute tolerance
snapshot := aligner.FindSnapshot(sortedReadings, targetTime)
```

## 📄 License
MIT
