package pipeline

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"observability-platform/internal/signal"

	"k8s.io/client-go/util/workqueue"
)

var ErrShuttingDown = errors.New("signal queue is shutting down")

type Handler func(context.Context, signal.Signal) error
type DeadLetterHandler func(context.Context, signal.Signal, error, int)

type Config struct {
	Workers    int
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

type Queue struct {
	queue      workqueue.TypedRateLimitingInterface[string]
	items      sync.Map
	handler    Handler
	deadLetter DeadLetterHandler
	workers    int
	maxRetries int
	closing    atomic.Bool
}

func NewQueue(cfg Config, handler Handler, deadLetter DeadLetterHandler) *Queue {
	limiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](cfg.BaseDelay, cfg.MaxDelay)
	return &Queue{queue: workqueue.NewTypedRateLimitingQueue(limiter), handler: handler, deadLetter: deadLetter, workers: cfg.Workers, maxRetries: cfg.MaxRetries}
}

func (q *Queue) Enqueue(s signal.Signal) error {
	if q.closing.Load() {
		return ErrShuttingDown
	}
	q.items.Store(s.ID, s)
	q.queue.Add(s.ID)
	return nil
}

func (q *Queue) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i < q.workers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); q.worker(ctx) }()
	}
	<-ctx.Done()
	q.closing.Store(true)
	q.queue.ShutDownWithDrain()
	wg.Wait()
	return nil
}

func (q *Queue) worker(ctx context.Context) {
	for {
		key, shutdown := q.queue.Get()
		if shutdown {
			return
		}
		func() {
			defer q.queue.Done(key)
			value, ok := q.items.Load(key)
			if !ok {
				q.queue.Forget(key)
				return
			}
			s := value.(signal.Signal)
			err := q.handler(ctx, s)
			if err == nil {
				q.queue.Forget(key)
				q.items.Delete(key)
				return
			}
			attempts := q.queue.NumRequeues(key) + 1
			if attempts <= q.maxRetries {
				q.queue.AddRateLimited(key)
				return
			}
			q.queue.Forget(key)
			q.items.Delete(key)
			if q.deadLetter != nil {
				q.deadLetter(ctx, s, err, attempts)
			}
		}()
	}
}

func (q *Queue) Depth() int { return q.queue.Len() }
