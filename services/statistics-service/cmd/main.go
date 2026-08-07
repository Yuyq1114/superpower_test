package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/pkg/servicehealth"
	"github.com/example/fitness-checkin/pkg/storage"
	statisticsv1 "github.com/example/fitness-checkin/proto/gen/statistics/v1"
	"github.com/example/fitness-checkin/services/statistics-service/internal/consumer"
	statisticsgrpc "github.com/example/fitness-checkin/services/statistics-service/internal/grpc"
	"github.com/example/fitness-checkin/services/statistics-service/internal/identity"
	"github.com/example/fitness-checkin/services/statistics-service/internal/repository"
	"github.com/example/fitness-checkin/services/statistics-service/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

func waitForStop(ctx context.Context, failures <-chan error) error {
	select {
	case err := <-failures:
		return err
	case <-ctx.Done():
		return nil
	}
}

func main() {
	if err := run(); err != nil {
		slog.Error("statistics service stopped", "error", err)
		os.Exit(1)
	}
}

func buildConsumer(r redis.UniversalClient, h consumer.Handler, name string, configure func(*consumer.Consumer)) (*consumer.Consumer, error) {
	c := consumer.New(r, h, name)
	if configure != nil {
		configure(c)
	}
	if err := c.ValidateSettings(); err != nil {
		return nil, fmt.Errorf("invalid consumer settings: %w", err)
	}
	return c, nil
}
func run() error {
	logger := observability.NewLogger("statistics-service", nil)
	slog.SetDefault(logger)
	cfg, err := config.Load("statistics-service")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	db, err := storage.OpenPostgres(startup, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	if err = repository.Migrate(startup, db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	rdb, err := storage.OpenRedis(startup, cfg)
	if err != nil {
		return fmt.Errorf("open redis: %w", err)
	}
	defer rdb.Close()
	reg := observability.NewRegistry()
	m := observability.NewMetrics(reg)
	cm := newConsumerMetrics(reg)
	svc := service.New(repository.GORM{DB: db, Schema: repository.DefaultSchema})
	c, err := buildConsumer(rdb, svc, consumerName(), nil)
	if err != nil {
		return err
	}
	c.Logger = logger
	c.OnConsumed = cm.consumed.Inc
	c.OnRetry = cm.retries.Inc
	c.OnDLQ = cm.dlq.Inc
	c.OnLag = func(value int64) { cm.lag.Set(float64(value)) }
	if err = c.EnsureGroup(startup); err != nil {
		return fmt.Errorf("ensure consumer group: %w", err)
	}
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(metricsInterceptor(m, logger), deadlineInterceptor(5*time.Second), identity.UnaryServerInterceptor(cfg.JWTSecret)), grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute, Time: 2 * time.Minute, Timeout: 20 * time.Second}))
	statisticsv1.RegisterStatisticsServiceServer(gs, statisticsgrpc.NewServer(svc))
	sqlDB, _ := db.DB()
	health := servicehealth.New(func(c context.Context) error {
		x, cancel := context.WithTimeout(c, 500*time.Millisecond)
		defer cancel()
		return sqlDB.PingContext(x)
	}, func(c context.Context) error {
		x, cancel := context.WithTimeout(c, 500*time.Millisecond)
		defer cancel()
		return rdb.Ping(x).Err()
	})
	healthv1.RegisterHealthServer(gs, health)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		sql, e := db.DB()
		if e != nil || !pingDB(r.Context(), sql) || !pingRedis(r.Context(), rdb) {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	failures := make(chan error, 3)
	health.SetServing(true)
	go func() {
		if e := c.Run(ctx); e != nil && ctx.Err() == nil {
			health.SetServing(false)
			failures <- fmt.Errorf("consumer: %w", e)
		}
	}()
	go func() {
		if e := gs.Serve(lis); e != nil && ctx.Err() == nil {
			health.SetServing(false)
			failures <- fmt.Errorf("grpc: %w", e)
		}
	}()
	go func() {
		if e := hs.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			failures <- fmt.Errorf("http: %w", e)
		}
	}()
	runtimeErr := waitForStop(ctx, failures)
	health.SetServing(false)
	cancel()
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = hs.Shutdown(shutdown)
	gs.GracefulStop()
	_ = lis.Close()
	return runtimeErr
}

type consumerMetrics struct {
	consumed, retries, dlq prometheus.Counter
	lag                    prometheus.Gauge
}

func newConsumerMetrics(reg prometheus.Registerer) *consumerMetrics {
	m := &consumerMetrics{consumed: prometheus.NewCounter(prometheus.CounterOpts{Name: "fitness_checkin_statistics_events_consumed_total", Help: "Successfully consumed workout events."}), retries: prometheus.NewCounter(prometheus.CounterOpts{Name: "fitness_checkin_statistics_event_retries_total", Help: "Workout event retries."}), dlq: prometheus.NewCounter(prometheus.CounterOpts{Name: "fitness_checkin_statistics_event_dlq_total", Help: "Workout events moved to DLQ."}), lag: prometheus.NewGauge(prometheus.GaugeOpts{Name: "fitness_checkin_statistics_consumer_lag", Help: "Pending workout events."})}
	reg.MustRegister(m.consumed, m.retries, m.dlq, m.lag)
	return m
}
func consumerName() string {
	if h, e := os.Hostname(); e == nil && h != "" {
		return h
	}
	return fmt.Sprintf("statistics-%d", os.Getpid())
}
func pingDB(ctx context.Context, db interface{ PingContext(context.Context) error }) bool {
	c, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return db.PingContext(c) == nil
}
func pingRedis(ctx context.Context, r redis.UniversalClient) bool {
	c, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return r.Ping(c).Err() == nil
}
func deadlineInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		c, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return h(c, req)
	}
}
func metricsInterceptor(m *observability.Metrics, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		started := time.Now()
		ctx = identity.WithRequestContext(ctx)
		user, trace, request, _ := identity.FromContext(ctx)
		if trace == "" {
			trace = "unauthenticated"
		}
		if request == "" {
			request = "unauthenticated"
		}
		m.RequestsTotal.WithLabelValues("statistics-service", info.FullMethod).Inc()
		out, err := h(ctx, req)
		user, trace, request, _ = identity.FromContext(ctx)
		m.DurationSeconds.WithLabelValues("statistics-service", info.FullMethod).Observe(time.Since(started).Seconds())
		if err != nil {
			m.ErrorsTotal.WithLabelValues("statistics-service", info.FullMethod).Inc()
		}
		logger.InfoContext(ctx, "request completed", "trace_id", trace, "request_id", request, "user_id", user, "method", info.FullMethod, "error", err)
		return out, err
	}
}
