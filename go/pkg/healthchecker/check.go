package healthchecker

import (
	"context"
	"github.com/jc-lab/distworker/go/pkg/api"
	"github.com/jc-lab/distworker/go/pkg/types"
	"sync"
	"time"
)

type Checkable interface {
	GetName() string
	Health(ctx context.Context) error
	IsRequirement() bool
}

type Feature struct {
	Name        string
	HealthFunc  func(ctx context.Context) error
	Requirement bool
}

func (f *Feature) GetName() string {
	return f.Name
}

func (f *Feature) Health(ctx context.Context) error {
	return f.HealthFunc(ctx)
}

func (f *Feature) IsRequirement() bool {
	return f.Requirement
}

func Check(ctx context.Context, features []Checkable, timeout time.Duration) (map[string]*api.HealthDetail, types.HealthStatus) {
	var mutex sync.Mutex
	var wg sync.WaitGroup

	status := types.HealthStatusUp
	details := make(map[string]*api.HealthDetail)

	for _, feature := range features {
		wg.Add(1)
		go func(feature Checkable) {
			defer wg.Done()

			subCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			startedAt := time.Now()
			err := feature.Health(subCtx)
			responseTime := time.Since(startedAt)

			detail := &api.HealthDetail{
				ResponseTime: responseTime.Milliseconds(),
			}

			if err != nil {
				detail.Status = types.HealthStatusDown
				message := err.Error()
				detail.Message = &message

				if feature.IsRequirement() {
					status = types.HealthStatusDown
				} else {
					status = types.HealthStatusDegraded
				}
			} else {
				detail.Status = types.HealthStatusUp
			}

			mutex.Lock()
			details[feature.GetName()] = detail
			mutex.Unlock()
		}(feature)
	}

	wg.Wait()

	return details, status
}
