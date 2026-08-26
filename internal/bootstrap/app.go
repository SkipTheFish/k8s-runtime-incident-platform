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
	"observability-platform/internal/datasource/prometheus"
	"observability-platform/internal/handler"
	"observability-platform/internal/incident"
	"observability-platform/internal/middleware"
	"observability-platform/internal/operator"
	"observability-platform/internal/query"
	logsquery "observability-platform/internal/query/logs"
	metricsquery "observability-platform/internal/query/metrics"
	tracesquery "observability-platform/internal/query/traces"
	"observability-platform/internal/repository"
	"observability-platform/internal/runtime/correlation"
	runtimeevent "observability-platform/internal/runtime/event"
	runtimegraph "observability-platform/internal/runtime/graph"
	"observability-platform/internal/runtime/incidentfsm"
	runtimeinformer "observability-platform/internal/runtime/informer"
	runtimeprom "observability-platform/internal/runtime/prometheus"
	runtimesync "observability-platform/internal/runtime/sync"
	runtimeline "observability-platform/internal/runtime/timeline"
	"observability-platform/internal/timeline"

	"github.com/gin-gonic/gin"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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
	if err := repository.InitDB(cfg.Postgres.DSN); err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	migrationCtx, cancelMigration := context.WithTimeout(context.Background(), cfg.Postgres.Timeout)
	defer cancelMigration()
	if err := runMigrations(migrationCtx); err != nil {
		repository.DB.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	router, engine := buildRouter(cfg)
	app := &App{cfg: cfg, server: &http.Server{Addr: fmt.Sprintf(":%d", cfg.App.Port), Handler: router}}
	app.components = append(app.components, componentFunc{name: "alert engine", run: func(ctx context.Context) error {
		engine.Start(ctx)
		return nil
	}})
	if cfg.KubernetesEnabled {
		components, err := kubernetesComponents(cfg)
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

func buildRouter(cfg *config.Config) (*gin.Engine, *alert.Engine) {
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

	r := gin.Default()
	r.GET("/health", handler.Health)
	api := r.Group("/api/v1", middleware.TenantMiddleware())
	logs := api.Group("/logs")
	logs.GET("/query", logsHandler.Query)
	logs.GET("/stats", logsHandler.Stats)
	logs.GET("/tail", logsquery.TailHandler)
	traces := api.Group("/traces")
	traces.GET("/search", tracesHandler.Search)
	traces.GET("/:traceId", tracesHandler.GetTrace)
	api.POST("/alerts/policy", alertHandler.CreatePolicy)
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

func kubernetesComponents(cfg *config.Config) ([]Component, error) {
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
	if err := (&obsv1.AlertPolicy{}).SetupWebhookWithManager(mgr); err != nil {
		return nil, fmt.Errorf("register AlertPolicy webhook: %w", err)
	}
	bus := runtimeevent.NewBus()
	graphRepo := runtimegraph.NewRepository()
	correlationEngine := correlation.NewEngine(bus, incidentfsm.NewService(incidentfsm.NewRepository(), runtimeline.NewRepository()), graphRepo)
	informer := runtimeinformer.NewK8sEventInformer(restConfig, bus, graphRepo)
	syncer := runtimesync.NewAlertSyncer(mgr.GetClient(), runtimeprom.NewClient(cfg.PrometheusURL), bus)
	return []Component{
		componentFunc{name: "controller manager", run: mgr.Start},
		componentFunc{name: "informer", run: informer.Start},
		componentFunc{name: "worker", run: func(ctx context.Context) error { correlationEngine.Start(ctx); return nil }},
		componentFunc{name: "dispatcher", run: func(ctx context.Context) error { syncer.Start(ctx); return nil }},
	}, nil
}
