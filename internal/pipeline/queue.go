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
	metrics    *Metrics
}

func NewQueue(cfg Config, handler Handler, deadLetter DeadLetterHandler) *Queue {
	limiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](cfg.BaseDelay, cfg.MaxDelay)
	return &Queue{queue: workqueue.NewTypedRateLimitingQueue(limiter), handler: handler, deadLetter: deadLetter, workers: cfg.Workers, maxRetries: cfg.MaxRetries}
}

func (q *Queue) SetMetrics(metrics *Metrics) { q.metrics = metrics }

func (q *Queue) Enqueue(s signal.Signal) error {
	if q.closing.Load() {
		return ErrShuttingDown
	}
	q.items.Store(s.ID, s)
	q.queue.Add(s.ID)
	if q.metrics != nil {
		q.metrics.received.With(labels(string(s.Source), string(s.Type))).Inc()
	}
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
			if errors.Is(err, ErrDuplicate) {
				q.queue.Forget(key)
				q.items.Delete(key)
				if q.metrics != nil {
					q.metrics.deduplicated.With(labels(string(s.Source), string(s.Type))).Inc()
				}
				return
			}
			attempts := q.queue.NumRequeues(key) + 1
			if attempts <= q.maxRetries {
				if q.metrics != nil {
					q.metrics.retries.With(labels(string(s.Source), string(s.Type))).Inc()
				}
				q.queue.AddRateLimited(key)
				return
			}
			q.queue.Forget(key)
			q.items.Delete(key)
			if q.metrics != nil {
				q.metrics.failed.With(labels(string(s.Source), string(s.Type))).Inc()
			}
			if q.deadLetter != nil {
				q.deadLetter(ctx, s, err, attempts)
			}
		}()
	}
}

func (q *Queue) Depth() int { return q.queue.Len() }
