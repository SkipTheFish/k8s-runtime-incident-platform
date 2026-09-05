package resourcegraph

import (
	"context"
	"fmt"
	"sync"
)

type ApplyFunc func(context.Context, ResourceTask) error

type Processor struct {
	queue   chan ResourceTask
	workers int
	apply   ApplyFunc
}

func NewProcessor(workers, capacity int, apply ApplyFunc) *Processor {
	if workers < 1 {
		workers = 1
	}
	if capacity < 1 {
		capacity = 256
	}
	return &Processor{queue: make(chan ResourceTask, capacity), workers: workers, apply: apply}
}

func (p *Processor) Enqueue(task ResourceTask) error {
	select {
	case p.queue <- task:
		return nil
	default:
		return fmt.Errorf("resource graph queue is full")
	}
}

func (p *Processor) Run(ctx context.Context) error {
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	errs := make(chan error, p.workers)
	for range p.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case task := <-p.queue:
					if err := p.apply(workerCtx, task); err != nil && workerCtx.Err() == nil {
						select {
						case errs <- err:
						default:
						}
						cancel()
						return
					}
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case err := <-errs:
		return fmt.Errorf("process resource graph task: %w", err)
	case <-ctx.Done():
		<-done
		return ctx.Err()
	case <-done:
		return nil
	}
}
