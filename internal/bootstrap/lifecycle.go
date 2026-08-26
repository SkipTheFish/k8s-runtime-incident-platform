package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Component interface {
	Name() string
	Run(context.Context) error
}

type componentFunc struct {
	name string
	run  func(context.Context) error
}

func (c componentFunc) Name() string                  { return c.name }
func (c componentFunc) Run(ctx context.Context) error { return c.run(ctx) }

func runComponents(ctx context.Context, timeout time.Duration, components []Component) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, len(components))
	var wg sync.WaitGroup
	for _, component := range components {
		component := component
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := component.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				results <- fmt.Errorf("%s: %w", component.Name(), err)
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	var runErr error
	select {
	case <-ctx.Done():
		runErr = context.Cause(ctx)
	case runErr = <-results:
		cancel()
	case <-done:
		return nil
	}
	cancel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		if errors.Is(runErr, context.Canceled) {
			return nil
		}
		return runErr
	case <-timer.C:
		return fmt.Errorf("shutdown exceeded %s", timeout)
	}
}
