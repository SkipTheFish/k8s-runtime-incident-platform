package main

import (
	"context"
	"fmt"
	"log"

	obsv1 "observability-platform/api/v1"
	"observability-platform/internal/alert"
	"observability-platform/internal/cache"
	"observability-platform/internal/config"
	"observability-platform/internal/datasource/prometheus"
	"observability-platform/internal/handler"
	"observability-platform/internal/incident"
	"observability-platform/internal/logger"
	"observability-platform/internal/middleware"
	"observability-platform/internal/operator"
	"observability-platform/internal/query"
	logsquery "observability-platform/internal/query/logs"
	metricsquery "observability-platform/internal/query/metrics"
	tracesquery "observability-platform/internal/query/traces"
	"observability-platform/internal/repository"
	"observability-platform/internal/timeline"

	"observability-platform/internal/runtime/correlation"
	runtimeevent "observability-platform/internal/runtime/event"
	runtimegraph "observability-platform/internal/runtime/graph"
	"observability-platform/internal/runtime/incidentfsm"
	runtimeinformer "observability-platform/internal/runtime/informer"
	runtimeprom "observability-platform/internal/runtime/prometheus"
	runtimesync "observability-platform/internal/runtime/sync"
	runtimeline "observability-platform/internal/runtime/timeline"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/gin-gonic/gin"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	log.Println("[INIT] 开始注册 Kubernetes Scheme...")

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	log.Println("[INIT] client-go Scheme 注册成功")

	utilruntime.Must(obsv1.AddToScheme(scheme))
	log.Println("[INIT] observability-platform CRD Scheme 注册成功")

	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	log.Println("[INIT] prometheus-operator Scheme 注册成功")

	log.Println("[INIT] Scheme 初始化运行成功")
}

func main() {
	ctx := context.Background()

	log.Println("========== BOOT START ==========")

	// ===============================
	// 1️⃣ 基础初始化
	// ===============================
	log.Println("[1] 开始初始化配置 config.Init()...")
	if err := config.Init(); err != nil {
		log.Fatalf("[1] 配置加载失败: %v", err)
	}
	log.Println("[1] 配置初始化运行成功")

	log.Println("[1] 开始连接数据库 repository.InitDB()...")
	if err := repository.InitDB(config.Cfg.Postgres.DSN); err != nil {
		log.Fatalf("[1] 数据库连接失败: %v", err)
	}
	log.Println("[1] 数据库连接运行成功")

	log.Println("[1] 开始执行数据库迁移 runMigrations()...")
	runMigrations()
	log.Println("[1] 数据库迁移运行成功")

	log.Println("[1] 开始初始化 logger.Init()...")
	logger.Init()
	log.Println("[1] logger 初始化运行成功")

	// ===============================
	// 2️⃣ 缓存
	// ===============================
	log.Println("[2] 开始初始化 Redis Cache...")
	redisCache := cache.NewRedis(config.Cfg.Redis.Addr)
	log.Println("[2] Redis Cache 初始化运行成功")

	log.Println("[2] 开始初始化 Query Cache...")
	queryCache := query.NewRedisQueryCache(redisCache)
	log.Println("[2] Query Cache 初始化运行成功")

	// ===============================
	// 3️⃣ Metrics / Logs / Traces
	// ===============================
	log.Println("[3] 开始初始化 Metrics / Logs / Traces...")

	// Prometheus
	log.Println("[3-Metrics] 开始初始化 Prometheus Client...")
	promClient := prometheus.NewClient("http://localhost:19090")
	log.Println("[3-Metrics] Prometheus Client 初始化运行成功")

	log.Println("[3-Metrics] 开始初始化 Prometheus Executor...")
	promExecutor := metricsquery.NewPrometheusExecutor(promClient)
	log.Println("[3-Metrics] Prometheus Executor 初始化运行成功")

	log.Println("[3-Metrics] 开始初始化 Metrics Pipeline...")
	metricsPipeline := query.NewQueryPipeline(
		&metricsquery.PromQLValidator{},
		&query.NoopParser{},
		&query.NoopIsolator{},
		queryCache,
		promExecutor,
		&query.DefaultFormatter{},
	)
	log.Println("[3-Metrics] Metrics Pipeline 初始化运行成功")

	log.Println("[3-Metrics] 开始初始化 Metrics Service / Handler...")
	metricsService := metricsquery.NewMetricsService(metricsPipeline)
	metricsHandler := metricsquery.NewHandler(metricsService)
	log.Println("[3-Metrics] Metrics Service / Handler 初始化运行成功")

	// Loki
	log.Println("[3-Logs] 开始初始化 Loki Client...")
	lokiClient := logsquery.NewLokiClient("http://localhost:3100")
	log.Println("[3-Logs] Loki Client 初始化运行成功")

	log.Println("[3-Logs] 开始初始化 Loki Executor...")
	lokiExecutor := logsquery.NewLokiExecutor(lokiClient)
	log.Println("[3-Logs] Loki Executor 初始化运行成功")

	log.Println("[3-Logs] 开始初始化 Logs Pipeline...")
	logsPipeline := query.NewQueryPipeline(
		&logsquery.LogQLValidator{},
		&query.NoopParser{},
		&query.NoopIsolator{},
		queryCache,
		lokiExecutor,
		&query.DefaultFormatter{},
	)
	log.Println("[3-Logs] Logs Pipeline 初始化运行成功")

	log.Println("[3-Logs] 开始初始化 Logs Service / Handler...")
	logsService := logsquery.NewLogsService(logsPipeline)
	logsHandler := logsquery.NewHandler(logsService)
	log.Println("[3-Logs] Logs Service / Handler 初始化运行成功")

	// Tempo
	log.Println("[3-Traces] 开始初始化 Tempo Client...")
	tempoClient := tracesquery.NewTempoClient("http://localhost:3200")
	log.Println("[3-Traces] Tempo Client 初始化运行成功")

	log.Println("[3-Traces] 开始初始化 Tempo Executor...")
	tempoExecutor := tracesquery.NewTempoExecutor(tempoClient)
	log.Println("[3-Traces] Tempo Executor 初始化运行成功")

	log.Println("[3-Traces] 开始初始化 Traces Pipeline...")
	tracesPipeline := query.NewQueryPipeline(
		&tracesquery.TraceValidator{},
		&query.NoopParser{},
		&query.NoopIsolator{},
		queryCache,
		tempoExecutor,
		&query.DefaultFormatter{},
	)
	log.Println("[3-Traces] Traces Pipeline 初始化运行成功")

	log.Println("[3-Traces] 开始初始化 Traces Service / Handler...")
	tracesService := tracesquery.NewTracesService(tracesPipeline)
	tracesHandler := tracesquery.NewHandler(tracesService)
	log.Println("[3-Traces] Traces Service / Handler 初始化运行成功")

	log.Println("[3] Metrics / Logs / Traces 全部初始化运行成功")

	// ===============================
	// 4️⃣ Alert / Incident
	// ===============================
	log.Println("[4] 开始初始化 Alert Repo / Service / Handler...")
	alertRepo := alert.NewPGRepo()
	alertService := alert.NewService(alertRepo)
	alertHandler := alert.NewHandler(alertService)
	log.Println("[4] Alert Repo / Service / Handler 初始化运行成功")

	log.Println("[4] 开始初始化 Incident Repo / Timeline Repo...")
	incidentRepo := incident.NewPGRepo()
	timelineRepo := timeline.NewPGRepo()

	log.Println("[4] 开始初始化 Timeline Service / Timeline Handler...")
	timelineService := timeline.NewService(
		timelineRepo,
	)
	timelineHandler := timeline.NewHandler(
		timelineService,
	)

	log.Println("[4] Incident Repo / Timeline Repo 初始化运行成功")

	log.Println("[4] 开始初始化 Incident Service / Handler...")
	// incidentService := incident.NewService(incidentRepo, timelineRepo)
	incidentService := incident.NewService(
		incidentRepo,
		timelineRepo,
	)
	incidentHandler := incident.NewHandler(
		incidentService,
	)
	log.Println("[4] Incident Service / Handler 初始化运行成功")

	// ===============================
	// 5️⃣ Alert Engine（旧逻辑）
	// ===============================
	log.Println("[5] 开始初始化 Alert Engine...")
	engine := alert.NewEngine(metricsService, alertRepo, incidentService)
	log.Println("[5] Alert Engine 初始化运行成功")

	log.Println("[5] 开始启动 Alert Engine...")
	go func() {

		log.Println("[5-Goroutine] Alert Engine 开始运行...")

		engine.Start(ctx)

		log.Println("[5-Goroutine] Alert Engine 已退出")

	}()
	log.Println("[5] Alert Engine Start() 已调用运行成功")

	// ===============================
	// 6️⃣ Kubernetes Controller Manager
	// ===============================
	log.Println("[6] 开始创建 Kubernetes Controller Manager...")
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: ":8081",
		},
	})
	if err != nil {
		log.Fatalf("[6] 创建 Kubernetes Controller Manager 失败: %v", err)
	}
	log.Println("[6] Kubernetes Controller Manager 创建运行成功")

	log.Println("[6] 开始初始化 Operator Repository...")
	opRepo := operator.NewPGRepository()
	log.Println("[6] Operator Repository 初始化运行成功")

	log.Println("[6] 开始注册 AlertPolicy Reconciler...")
	if err := (&operator.AlertPolicyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Repo:   opRepo,
	}).SetupWithManager(mgr); err != nil {
		log.Fatalf("[6] 注册 AlertPolicy Reconciler 失败: %v", err)
	}
	log.Println("[6] AlertPolicy Reconciler 注册运行成功")

	log.Println("[6] 开始注册 AlertPolicy Webhook...")
	if err := (&obsv1.AlertPolicy{}).SetupWebhookWithManager(mgr); err != nil {
		log.Fatalf("[6] 注册 AlertPolicy Webhook 失败: %v", err)
	}
	log.Println("[6] AlertPolicy Webhook 注册运行成功")

	log.Println("[6] 准备异步启动 Kubernetes Controller Manager...")
	go func() {
		log.Println("[6-Goroutine] Kubernetes Controller Manager 开始运行...")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			log.Fatalf("[6-Goroutine] Kubernetes Controller Manager 运行失败: %v", err)
		}
		log.Println("[6-Goroutine] Kubernetes Controller Manager 已退出")
	}()
	log.Println("[6] Kubernetes Controller Manager goroutine 启动运行成功")

	// ===============================
	// 7️⃣ Runtime Event Bus / Correlation / Sync
	// ===============================

	log.Println("[7] 开始初始化 Runtime Event Bus...")
	runtimeBus := runtimeevent.NewBus()
	log.Println("[7] Runtime Event Bus 初始化运行成功")

	// ---------------------------------
	// Runtime Timeline / Incident FSM
	// ---------------------------------

	log.Println("[7] 开始初始化 Runtime Timeline Repository...")
	runtimeTimelineRepo := runtimeline.NewRepository()
	log.Println("[7] Runtime Timeline Repository 初始化运行成功")

	// log.Println("[7] 开始初始化 Runtime Timeline API...")
	// timelineAPI := runtimeline.NewAPI(
	// 	runtimeTimelineRepo,
	// )

	// log.Println("[7] Runtime Timeline API 初始化运行成功")

	log.Println("[7] 开始初始化 Incident FSM Repository...")
	incidentFSMRepo := incidentfsm.NewRepository()
	log.Println("[7] Incident FSM Repository 初始化运行成功")

	log.Println("[7] 开始初始化 Incident FSM Service...")
	incidentFSM := incidentfsm.NewService(
		incidentFSMRepo,
		runtimeTimelineRepo,
	)
	log.Println("[7] Incident FSM Service 初始化运行成功")

	// ---------------------------------
	// Runtime Graph
	// ---------------------------------

	log.Println("[7] 开始初始化 Runtime Graph Repository...")
	runtimeGraphRepo := runtimegraph.NewRepository()
	log.Println("[7] Runtime Graph Repository 初始化运行成功")

	// ---------------------------------
	// Correlation Engine
	// ---------------------------------

	log.Println("[7] 开始初始化 Correlation Engine...")
	correlationEngine := correlation.NewEngine(
		runtimeBus,
		incidentFSM,
		runtimeGraphRepo,
	)
	log.Println("[7] Correlation Engine 初始化运行成功")

	log.Println("[7] 准备异步启动 Correlation Engine...")
	go func() {
		log.Println("[7-Goroutine] Correlation Engine 开始运行...")
		correlationEngine.Start(ctx)
		log.Println("[7-Goroutine] Correlation Engine 已退出")
	}()

	// ---------------------------------
	// Kubernetes Event Informer
	// ---------------------------------

	log.Println("[7] 开始初始化 Kubernetes Event Informer...")
	k8sEventInformer := runtimeinformer.NewK8sEventInformer(
		ctrl.GetConfigOrDie(),
		runtimeBus,
		runtimeGraphRepo, // 确认保留三个参数
	)
	log.Println("[7] Kubernetes Event Informer 初始化运行成功")

	log.Println("[7] 准备异步启动 Kubernetes Event Informer...")
	go func() {
		log.Println("[7-Goroutine] Kubernetes Event Informer 开始运行...")
		if err := k8sEventInformer.Start(ctx); err != nil {
			// 使用 log.Fatalf 可以在启动失败时直接终止整个程序，防止带着故障运行
			log.Fatalf("[7-Goroutine] Kubernetes Event Informer 运行失败: %v", err)
		}
		log.Println("[7-Goroutine] Kubernetes Event Informer 已退出")
	}()

	// ---------------------------------
	// Prometheus Runtime Client
	// ---------------------------------

	log.Println("[7] 开始初始化 Prometheus Runtime Client...")
	promRuntimeClient := runtimeprom.NewClient("http://localhost:19090")
	log.Println("[7] Prometheus Runtime Client 初始化运行成功")

	// ---------------------------------
	// Alert Syncer
	// ---------------------------------

	log.Println("[7] 开始初始化 Alert Syncer...")
	alertSyncer := runtimesync.NewAlertSyncer(
		mgr.GetClient(),
		promRuntimeClient,
		runtimeBus,
	)
	log.Println("[7] Alert Syncer 初始化运行成功")

	log.Println("[7] 准备异步启动 Alert Syncer...")
	go func() {
		log.Println("[7-Goroutine] Alert Syncer 开始运行...")
		alertSyncer.Start(ctx)
		log.Println("[7-Goroutine] Alert Syncer 已退出")
	}()

	log.Println("[7] 所有 Runtime 核心组件异步启动指令已下发完成")

	// ===============================
	// 8️⃣ Gin API
	// ===============================
	log.Println("[8] 开始初始化 Gin Router...")
	r := gin.Default()
	log.Println("[8] Gin Router 初始化运行成功")

	// log.Println("[8] 开始注册 Runtime Timeline API...")
	// timelineAPI.Register(r)
	// log.Println("[8] Runtime Timeline API 注册运行成功")

	log.Println("[8] 开始注册 /health 路由...")
	r.GET("/health", handler.Health)
	log.Println("[8] /health 路由注册运行成功")

	log.Println("[8] 开始注册 /api/v1 路由组和 TenantMiddleware...")
	api := r.Group("/api/v1")
	api.Use(middleware.TenantMiddleware())
	log.Println("[8] /api/v1 路由组注册运行成功")

	log.Println("[8] 开始注册 Logs 路由...")
	logsGroup := api.Group("/logs")
	{
		logsGroup.GET("/query", logsHandler.Query)
		logsGroup.GET("/stats", logsHandler.Stats)
		logsGroup.GET("/tail", logsquery.TailHandler)
	}
	log.Println("[8] Logs 路由注册运行成功")

	log.Println("[8] 开始注册 Traces 路由...")
	tracesGroup := api.Group("/traces")
	{
		tracesGroup.GET("/search", tracesHandler.Search)
		tracesGroup.GET("/:traceId", tracesHandler.GetTrace)
	}
	log.Println("[8] Traces 路由注册运行成功")

	log.Println("[8] 开始注册 Alerts 路由...")
	alertGroup := api.Group("/alerts")
	{
		alertGroup.POST("/policy", alertHandler.CreatePolicy)
	}
	log.Println("[8] Alerts 路由注册运行成功")

	log.Println("[8] 开始注册 Incidents 路由...")
	incidentGroup := api.Group("/incidents")
	{
		incidentGroup.GET("", incidentHandler.List)

		incidentGroup.GET("/open", incidentHandler.ListOpen)

		incidentGroup.GET("/:id", incidentHandler.Get)

		incidentGroup.GET(
			"/:id/timeline",
			timelineHandler.GetByIncident,
		)
	}
	log.Println("[8] Incidents 路由注册运行成功")

	log.Println("[8] 开始注册 Metrics 路由...")
	metricsGroup := api.Group("/metrics")
	{
		metricsGroup.GET("/query", metricsHandler.Query)
		metricsGroup.GET("/range", metricsHandler.Range)
		metricsGroup.GET("/golden-signals", metricsHandler.GoldenSignals)
		metricsGroup.GET("/labels", metricsHandler.Labels)
	}
	log.Println("[8] Metrics 路由注册运行成功")

	log.Println("[8] Gin API 路由全部注册运行成功")

	// ===============================
	// 9️⃣ 启动 HTTP
	// ===============================
	addr := fmt.Sprintf(":%d", config.Cfg.App.Port)
	log.Printf("[9] 准备启动 HTTP Server，端口: %d", config.Cfg.App.Port)

	log.Println("========== BOOT SUCCESS: 即将进入 r.Run() 阻塞监听 ==========")

	if err := r.Run(addr); err != nil {
		log.Fatalf("[9] HTTP Server 启动失败: %v", err)
	}
}

func runMigrations() {
	log.Println("[MIGRATION] 开始执行 SQL...")

	_, err := repository.DB.Exec(context.Background(), `
CREATE TABLE IF NOT EXISTS alert_policies (
    id SERIAL PRIMARY KEY,
    tenant_id TEXT,
    name TEXT,
    query TEXT,
    threshold DOUBLE PRECISION,
    operator TEXT,
    duration string,
    created_at TIMESTAMP DEFAULT NOW(),
    k8s_namespace TEXT,
    k8s_name TEXT
);

CREATE TABLE IF NOT EXISTS alert_instances (
    id SERIAL PRIMARY KEY,
    policy_id INT,
    status TEXT,
    value DOUBLE PRECISION,
    start_time TIMESTAMP,
    end_time TIMESTAMP
);

CREATE TABLE IF NOT EXISTS incidents (
    id SERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    namespace TEXT NOT NULL,
    policy_name TEXT NOT NULL,
    alert_name TEXT,
    severity TEXT,
    status TEXT NOT NULL,
    started_at TIMESTAMP DEFAULT NOW(),
    resolved_at TIMESTAMP,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_open_unique
ON incidents (tenant_id, namespace, policy_name)
WHERE status = 'open';

CREATE TABLE IF NOT EXISTS runtime_timeline (
    id SERIAL PRIMARY KEY,
    incident_id INT,
    tenant_id TEXT,
    namespace TEXT,
    source TEXT,
    event_type TEXT,
    resource_kind TEXT,
    resource_name TEXT,
    policy_name TEXT,
    message TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS runtime_graph_nodes (
    id SERIAL PRIMARY KEY,
    node_key TEXT UNIQUE NOT NULL,
    kind TEXT NOT NULL,
    namespace TEXT,
    name TEXT NOT NULL,
    tenant_id TEXT,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS runtime_graph_edges (
    id SERIAL PRIMARY KEY,
    from_key TEXT NOT NULL,
    to_key TEXT NOT NULL,
    edge_type TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(from_key, to_key, edge_type)
);

-- phase9B - 3
ALTER TABLE incidents
ADD COLUMN IF NOT EXISTS resource_kind TEXT;

ALTER TABLE incidents
ADD COLUMN IF NOT EXISTS resource_name TEXT;

ALTER TABLE incidents
ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMP;

ALTER TABLE incidents
ADD COLUMN IF NOT EXISTS suppressed_at TIMESTAMP;

ALTER TABLE incidents
ADD COLUMN IF NOT EXISTS reopen_count INT DEFAULT 0;

ALTER TABLE incidents
ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMP DEFAULT NOW();



`)
	if err != nil {
		log.Fatalf("[MIGRATION] Migration failed: %v", err)
	}

	log.Println("[MIGRATION] SQL 执行运行成功")
}
