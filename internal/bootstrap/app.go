package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/alert"
	"observability-platform/internal/cache"
	"observability-platform/internal/config"
	correlationrules "observability-platform/internal/correlation"
	"observability-platform/internal/database"
	"observability-platform/internal/datasource/prometheus"
	"observability-platform/internal/handler"
	"observability-platform/internal/incident"
	"observability-platform/internal/middleware"
	"observability-platform/internal/operator"
	"observability-platform/internal/pipeline"
	"observability-platform/internal/query"
	logsquery "observability-platform/internal/query/logs"
	metricsquery "observability-platform/internal/query/metrics"
	tracesquery "observability-platform/internal/query/traces"
	"observability-platform/internal/repository"
	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/runtime/correlation"
	runtimeevent "observability-platform/internal/runtime/event"
	runtimegraph "observability-platform/internal/runtime/graph"
	"observability-platform/internal/runtime/incidentfsm"
	runtimeinformer "observability-platform/internal/runtime/informer"
	runtimeprom "observability-platform/internal/runtime/prometheus"
	runtimesync "observability-platform/internal/runtime/sync"
	runtimeline "observability-platform/internal/runtime/timeline"
	"observability-platform/internal/signal"
	"observability-platform/internal/timeline"

	"github.com/gin-gonic/gin"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prommetrics "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type App struct {
	cfg        *config.Config
	server     *http.Server
	components []Component
}

func New(cfg *config.Config) (*App, error) {
	migrationDB, err := database.Open(cfg.Postgres.DSN)
	if err != nil {
		return nil, err
	}
	migrationCtx, cancelMigration := context.WithTimeout(context.Background(), cfg.Postgres.Timeout)
	defer cancelMigration()
	if err := database.Up(migrationCtx, migrationDB); err != nil {
		migrationDB.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	if err := migrationDB.Close(); err != nil {
		return nil, fmt.Errorf("close migration database: %w", err)
	}
	if err := repository.InitDB(cfg.Postgres.DSN); err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	readiness := handler.NewReadiness(repository.DB.Ping, cfg.KubernetesEnabled, cfg.Postgres.Timeout)
	readiness.SetMigrationsReady(true)

	ruleRegistry := correlationrules.NewRegistry(correlationrules.BuiltInRules()...)
	router, engine := buildRouter(cfg, readiness, ruleRegistry)
	app := &App{cfg: cfg, server: &http.Server{Addr: fmt.Sprintf(":%d", cfg.App.Port), Handler: router}}
	app.components = append(app.components, componentFunc{name: "alert engine", run: func(ctx context.Context) error {
		engine.Start(ctx)
		return nil
	}})
	if cfg.KubernetesEnabled {
		components, err := kubernetesComponents(cfg, readiness, ruleRegistry)
		if err != nil {
			repository.DB.Close()
			return nil, err
		}
		app.components = append(app.components, components...)
	}
	app.components = append(app.components, componentFunc{name: "http server", run: app.runHTTP})
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	defer repository.DB.Close()
	return runComponents(ctx, a.cfg.App.ShutdownTimeout, a.components)
}

func (a *App) runHTTP(ctx context.Context) error {
	result := make(chan error, 1)
	go func() { result <- a.server.ListenAndServe() }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}

func buildRouter(cfg *config.Config, readiness *handler.Readiness, ruleRegistry *correlationrules.Registry) (*gin.Engine, *alert.Engine) {
	redisCache := cache.NewRedis(cfg.Redis.Addr)
	queryCache := query.NewRedisQueryCache(redisCache)
	promClient := prometheus.NewClient(cfg.PrometheusURL)
	metricsPipeline := query.NewQueryPipeline(&metricsquery.PromQLValidator{}, &query.NoopParser{}, &query.NoopIsolator{}, queryCache, metricsquery.NewPrometheusExecutor(promClient), &query.DefaultFormatter{})
	metricsService := metricsquery.NewMetricsService(metricsPipeline)
	metricsHandler := metricsquery.NewHandler(metricsService)
	logsPipeline := query.NewQueryPipeline(&logsquery.LogQLValidator{}, &query.NoopParser{}, &query.NoopIsolator{}, queryCache, logsquery.NewLokiExecutor(logsquery.NewLokiClient(cfg.LokiURL)), &query.DefaultFormatter{})
	logsHandler := logsquery.NewHandler(logsquery.NewLogsService(logsPipeline))
	tracesPipeline := query.NewQueryPipeline(&tracesquery.TraceValidator{}, &query.NoopParser{}, &query.NoopIsolator{}, queryCache, tracesquery.NewTempoExecutor(tracesquery.NewTempoClient(cfg.TempoURL)), &query.DefaultFormatter{})
	tracesHandler := tracesquery.NewHandler(tracesquery.NewTracesService(tracesPipeline))

	alertRepo := alert.NewPGRepo()
	incidentRepo := incident.NewPGRepo()
	timelineRepo := timeline.NewPGRepo()
	incidentService := incident.NewService(incidentRepo, timelineRepo)
	alertHandler := alert.NewHandler(alert.NewService(alertRepo))
	incidentHandler := incident.NewHandler(incidentService)
	timelineHandler := timeline.NewHandler(timeline.NewService(timelineRepo))
	simulationHandler := correlationrules.NewSimulationHandler(correlationrules.NewSimulator(ruleRegistry, pipeline.NewRepository(repository.DB), 1000))

	r := gin.Default()
	r.GET("/health", handler.Health)
	r.GET("/livez", readiness.Live)
	r.GET("/readyz", readiness.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	api := r.Group("/api/v1", middleware.TenantMiddleware())
	logs := api.Group("/logs")
	logs.GET("/query", logsHandler.Query)
	logs.GET("/stats", logsHandler.Stats)
	logs.GET("/tail", logsquery.TailHandler)
	traces := api.Group("/traces")
	traces.GET("/search", tracesHandler.Search)
	traces.GET("/:traceId", tracesHandler.GetTrace)
	api.POST("/alerts/policy", alertHandler.CreatePolicy)
	api.POST("/correlation-policies/:name/simulate", simulationHandler.Simulate)
	incidents := api.Group("/incidents")
	incidents.GET("", incidentHandler.List)
	incidents.GET("/open", incidentHandler.ListOpen)
	incidents.GET("/:id", incidentHandler.Get)
	incidents.GET("/:id/timeline", timelineHandler.GetByIncident)
	metrics := api.Group("/metrics")
	metrics.GET("/query", metricsHandler.Query)
	metrics.GET("/range", metricsHandler.Range)
	metrics.GET("/golden-signals", metricsHandler.GoldenSignals)
	metrics.GET("/labels", metricsHandler.Labels)
	return r, alert.NewEngine(metricsService, alertRepo, incidentService)
}

func kubernetesComponents(cfg *config.Config, readiness *handler.Readiness, ruleRegistry *correlationrules.Registry) ([]Component, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(obsv1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("load Kubernetes configuration: %w", err)
	}
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: ":8081"}})
	if err != nil {
		return nil, fmt.Errorf("create controller manager: %w", err)
	}
	if err := (&operator.AlertPolicyReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Repo: operator.NewPGRepository()}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("register AlertPolicy controller: %w", err)
	}
	if err := (&operator.CorrelationPolicyReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Registry: ruleRegistry}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("register CorrelationPolicy controller: %w", err)
	}
	if err := (&obsv1.AlertPolicy{}).SetupWebhookWithManager(mgr); err != nil {
		return nil, fmt.Errorf("register AlertPolicy webhook: %w", err)
	}
	if err := (&obsv1.CorrelationPolicy{}).SetupWebhookWithManager(mgr); err != nil {
		return nil, fmt.Errorf("register CorrelationPolicy webhook: %w", err)
	}
	bus := runtimeevent.NewBus()
	correlationGraphRepo := runtimegraph.NewRepository()
	correlationEngine := correlation.NewEngine(bus, incidentfsm.NewService(incidentfsm.NewRepository(), runtimeline.NewRepository()), correlationGraphRepo)
	signalRepo := pipeline.NewRepository(repository.DB)
	graphHandler := pipeline.NewGraphHandler(signalRepo)
	graphHandler.SetEvaluator(ruleRegistry)
	graphHandler.SetCorrelationMetrics(pipeline.NewCorrelationMetrics(prommetrics.DefaultRegisterer))
	signalQueue := pipeline.NewQueue(pipeline.Config{Workers: cfg.SignalWorkers, MaxRetries: cfg.SignalMaxRetries, BaseDelay: cfg.SignalRetryBase, MaxDelay: cfg.SignalRetryMax}, graphHandler.Handle, func(ctx context.Context, item signal.Signal, cause error, attempts int) {
		_ = signalRepo.MarkUnmatched(ctx, item.ID, cause.Error())
		_ = signalRepo.RecordDeadLetter(ctx, item, "processing", cause, attempts)
	})
	signalQueue.SetMetrics(pipeline.NewMetrics(prommetrics.DefaultRegisterer, signalQueue.Depth))
	resourceGraphRepo := resourcegraph.NewRepository(repository.DB)
	graphProcessor := resourcegraph.NewProcessor(cfg.SignalWorkers, 1024, resourceGraphRepo.Apply)
	graphCleaner := resourcegraph.NewCleaner(cfg.ResourceGraph.Retention, cfg.ResourceGraph.CleanupInterval, cfg.ResourceGraph.CleanupTimeout, cfg.ResourceGraph.CleanupBatch, resourceGraphRepo.CleanExpired)
	informer := runtimeinformer.NewK8sEventInformer(restConfig, cfg.ClusterID, signalQueue.Enqueue, readiness.SetCacheSynced).WithResourceEnqueue(graphProcessor.Enqueue).WithResolver(graphHandler.SetResolver)
	syncer := runtimesync.NewAlertSyncer(mgr.GetClient(), runtimeprom.NewClient(cfg.PrometheusURL), bus)
	return []Component{
		componentFunc{name: "controller manager", run: mgr.Start},
		componentFunc{name: "informer", run: informer.Start},
		componentFunc{name: "signal worker", run: signalQueue.Run},
		componentFunc{name: "resource graph worker", run: graphProcessor.Run},
		componentFunc{name: "resource graph cleaner", run: graphCleaner.Run},
		componentFunc{name: "worker", run: func(ctx context.Context) error { correlationEngine.Start(ctx); return nil }},
		componentFunc{name: "dispatcher", run: func(ctx context.Context) error { syncer.Start(ctx); return nil }},
	}, nil
}
