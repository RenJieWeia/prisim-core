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
  - **Precision Control**: `Unifier` converts floating-point readings to high-precision integer scaled values (e.g., kWh to micro-kWh) to eliminate floating-point arithmetic errors.
  - **Time Alignment**: `Aligner` snaps readings to standard intervals (Snapshots).
- **Hexagonal Architecture**:
  - **Domain**: Pure business logic (`pkg/core/domain`), standard interfaces (`CleaningRule`, `Sanitizer`, `Unifier`).
  - **Ports**: Inbound (API/Ingestors) and Outbound (Repositories/Databases) definitions.
  - **Services**: Orchestration layer gluing domain logic to ports (`pkg/core/services`).

## 📂 Project Structure

```
prism-core/
├── pkg/
│   └── core/
│       ├── domain/        # Pure Business Logic (Entities & Rules)
│       │   ├── aligner.go
│       │   ├── sanitizer.go
│       │   ├── unifier.go
│       │   └── rules.go
│       ├── ports/         # Interface Definitions (Driver/Driven)
│       └── services/      # Application Services (Orchestration)
├── tests/                 # External Integration Tests
│   ├── core/
│   │   ├── domain/
│   │   └── services/
└── testdata/              # Sample data for tests
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

    // Import from the public package path
    "github.com/renjie/prism-core/pkg/adapters/ingest"
    "github.com/renjie/prism-core/pkg/core/services"
    "github.com/renjie/prism-core/pkg/core/domain"
)

func main() {
    // 1. Setup Ingestion
    ingestor := ingest.NewJsonUniversalIngestor(func(ctx context.Context, readings []domain.Reading) error {
        fmt.Printf("Received batch of %d readings\n", len(readings))
        return nil
    })
    
    // 2. Setup Standardization Service
    // Configure with 15-minute alignment and 4-decimal precision
    standardizer := services.NewCoreStandardizer(
        services.WithAlignment(15*time.Minute, 1*time.Minute),
        services.WithPrecision(10000),
    )
}
```

### Running Tests

```bash
go test ./tests/...
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
