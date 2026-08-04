package reference

import (
	"context"
	"sort"
	"time"

	"github.com/renjie/prism-core/pkg/core/domain"
	"github.com/renjie/prism-core/pkg/core/ports"
)

// RepositoryReferenceSource 标准数据仓储参考源
// 基于现有 StandardReadingRepository 查询历史标准数据作为参考对象。
// 查询以批量为单位执行，配合调用方的内存缓存可避免每条数据、每条规则单独查库。
type RepositoryReferenceSource struct {
	repo      ports.StandardReadingRepository
	tolerance time.Duration // RELATIVE 模式最近邻查找的容差
}

// NewRepositoryReferenceSource 创建仓储参考源
func NewRepositoryReferenceSource(repo ports.StandardReadingRepository, tolerance time.Duration) *RepositoryReferenceSource {
	return &RepositoryReferenceSource{repo: repo, tolerance: tolerance}
}

// Resolve 实现 ports.ReferenceSource 接口
func (s *RepositoryReferenceSource) Resolve(ctx context.Context, requests []domain.ReferenceRequest) (map[string]domain.ReferenceValue, error) {
	out := make(map[string]domain.ReferenceValue, len(requests))
	for _, req := range requests {
		val, err := s.resolveOne(ctx, req)
		if err != nil {
			return nil, err
		}
		out[req.ID] = val
	}
	return out, nil
}

func (s *RepositoryReferenceSource) resolveOne(ctx context.Context, req domain.ReferenceRequest) (domain.ReferenceValue, error) {
	switch req.Mode {
	case domain.ReferenceTimePrevious:
		return s.resolvePrevious(ctx, req)
	case domain.ReferenceTimeRelative:
		return s.resolveRelative(ctx, req)
	case domain.ReferenceTimeWindow:
		return s.resolveWindow(ctx, req)
	default:
		return domain.ReferenceValue{}, nil
	}
}

// resolvePrevious 查询当前时间之前最近的一条标准读数
func (s *RepositoryReferenceSource) resolvePrevious(ctx context.Context, req domain.ReferenceRequest) (domain.ReferenceValue, error) {
	// 以较早时间作为窗口起点，查询目标时间之前的所有记录
	start := time.Unix(0, 0)
	rows, err := s.repo.FindRange(ctx, req.DeviceID, start, req.Target)
	if err != nil {
		return domain.ReferenceValue{}, err
	}
	if len(rows) == 0 {
		return domain.ReferenceValue{}, nil
	}

	// 取时间最新的那条
	last := rows[len(rows)-1]
	return domain.ReferenceValue{
		Found:     true,
		Timestamp: last.Timestamp,
		Value:     last.ValueDisplay,
	}, nil
}

// resolveRelative 查询目标时间点附近的最近一条标准读数
// 使用请求中的 Tolerance 作为查找窗口; 请求未指定时回退到源自身的容差。
func (s *RepositoryReferenceSource) resolveRelative(ctx context.Context, req domain.ReferenceRequest) (domain.ReferenceValue, error) {
	tol := req.Tolerance
	if tol <= 0 {
		tol = s.tolerance
	}
	start := req.Target.Add(-tol)
	end := req.Target.Add(tol)
	rows, err := s.repo.FindRange(ctx, req.DeviceID, start, end)
	if err != nil {
		return domain.ReferenceValue{}, err
	}
	if len(rows) == 0 {
		return domain.ReferenceValue{}, nil
	}

	// 找到离目标时间最近的记录
	best := rows[0]
	bestDiff := durationAbs(best.Timestamp.Sub(req.Target))
	for _, r := range rows {
		if diff := durationAbs(r.Timestamp.Sub(req.Target)); diff < bestDiff {
			bestDiff = diff
			best = r
		}
	}
	return domain.ReferenceValue{
		Found:     true,
		Timestamp: best.Timestamp,
		Value:     best.ValueDisplay,
	}, nil
}

// resolveWindow 查询时间窗口内的标准读数并聚合
func (s *RepositoryReferenceSource) resolveWindow(ctx context.Context, req domain.ReferenceRequest) (domain.ReferenceValue, error) {
	rows, err := s.repo.FindRange(ctx, req.DeviceID, req.Start, req.End)
	if err != nil {
		return domain.ReferenceValue{}, err
	}
	if len(rows) == 0 {
		return domain.ReferenceValue{}, nil
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Timestamp.Before(rows[j].Timestamp)
	})

	points := make([]domain.ReferencePoint, 0, len(rows))
	for _, r := range rows {
		points = append(points, domain.ReferencePoint{Timestamp: r.Timestamp, Value: r.ValueDisplay})
	}

	val, ok := domain.ReducePoints(points, req.Reducer)
	if !ok {
		return domain.ReferenceValue{}, nil
	}
	return domain.ReferenceValue{
		Found:     true,
		Timestamp: points[len(points)-1].Timestamp,
		Value:     val,
		Points:    points,
	}, nil
}

func durationAbs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
