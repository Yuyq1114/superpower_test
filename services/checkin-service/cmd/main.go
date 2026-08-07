package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/pkg/servicehealth"
	"github.com/example/fitness-checkin/pkg/storage"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
	planv1 "github.com/example/fitness-checkin/proto/gen/plan/v1"
	"github.com/example/fitness-checkin/services/checkin-service/internal/events"
	checkingrpc "github.com/example/fitness-checkin/services/checkin-service/internal/grpc"
	"github.com/example/fitness-checkin/services/checkin-service/internal/identity"
	"github.com/example/fitness-checkin/services/checkin-service/internal/repository"
	"github.com/example/fitness-checkin/services/checkin-service/internal/service"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type planChecker struct{ client planv1.PlanServiceClient }

func (p planChecker) CheckWorkoutItem(ctx context.Context, u, i string) error {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ctx = metadata.NewOutgoingContext(ctx, md.Copy())
	}
	_, e := p.client.GetWorkoutItem(ctx, &planv1.GetWorkoutItemRequest{UserId: u, WorkoutItemId: i})
	return e
}
func main() {
	logger := observability.NewLogger("checkin-service", nil)
	slog.SetDefault(logger)
	cfg, e := config.Load("checkin-service")
	if e != nil {
		logger.Error("startup failed", "error", e)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cc, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	db, e := storage.OpenPostgres(cc, cfg)
	if e != nil {
		logger.Error("database failed", "error", e)
		os.Exit(1)
	}
	if e = repository.Migrate(cc, db); e != nil {
		logger.Error("migration failed", "error", e)
		os.Exit(1)
	}
	red, e := storage.OpenRedis(cc, cfg)
	if e != nil {
		logger.Error("redis failed", "error", e)
		os.Exit(1)
	}
	defer red.Close()
	planAddr := os.Getenv("PLAN_SERVICE_ADDR")
	if planAddr == "" {
		planAddr = "localhost:9091"
	}
	pc, e := grpc.DialContext(cc, planAddr, grpc.WithInsecure(), grpc.WithBlock())
	if e != nil {
		logger.Error("plan service failed", "error", e)
		os.Exit(1)
	}
	defer pc.Close()
	reg := observability.NewRegistry()
	m := observability.NewMetrics(reg)
	published := prometheus.NewCounter(prometheus.CounterOpts{Name: "fitness_checkin_outbox_published_total", Help: "Published outbox events."})
	failed := prometheus.NewCounter(prometheus.CounterOpts{Name: "fitness_checkin_outbox_failed_total", Help: "Failed outbox publishes."})
	retried := prometheus.NewCounter(prometheus.CounterOpts{Name: "fitness_checkin_outbox_retried_total", Help: "Retried outbox events."})
	reg.MustRegister(published, failed, retried)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(identity.UnaryServerInterceptor(cfg.JWTSecret), deadlineInterceptor(5*time.Second), metricsInterceptor(m, logger)), grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute, Time: 2 * time.Minute, Timeout: 20 * time.Second}), grpc.MaxRecvMsgSize(4<<20), grpc.MaxSendMsgSize(4<<20))
	checkinv1.RegisterCheckinServiceServer(gs, checkingrpc.NewServer(service.New(repository.GORM{DB: db, Schema: repository.DefaultSchema}, planChecker{client: planv1.NewPlanServiceClient(pc)})))
	sqlDB, _ := db.DB()
	health := servicehealth.New(func(c context.Context) error {
		x, cancel := context.WithTimeout(c, 500*time.Millisecond)
		defer cancel()
		return sqlDB.PingContext(x)
	}, func(c context.Context) error {
		x, cancel := context.WithTimeout(c, 500*time.Millisecond)
		defer cancel()
		return red.Ping(x).Err()
	})
	healthv1.RegisterHealthServer(gs, health)
	pub := &events.Publisher{Logger: logger, Published: published.Inc, Failed: failed.Inc, Retried: retried.Inc, Repo: repository.GORM{DB: db, Schema: repository.DefaultSchema}, Redis: red}
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if e := pub.PublishPending(ctx, 100); e != nil {
					logger.Error("event publish failed", "error", e)
				}
			}
		}
	}()
	lis, e := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if e != nil {
		logger.Error("listen failed", "error", e)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		sql, x := db.DB()
		if x != nil || !pingDB(r.Context(), sql) || !pingRedis(r.Context(), red) {
			http.Error(w, "not ready", 503)
			return
		}
		w.WriteHeader(200)
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	health.SetServing(true)
	go func() {
		if err := gs.Serve(lis); err != nil && ctx.Err() == nil {
			health.SetServing(false)
			logger.Error("grpc stopped", "error", err)
			cancel()
		}
	}()
	go func() {
		if x := hs.ListenAndServe(); x != nil && !errors.Is(x, http.ErrServerClosed) {
			logger.Error("http stopped", "error", x)
			cancel()
		}
	}()
	<-ctx.Done()
	sh, x := context.WithTimeout(context.Background(), 5*time.Second)
	if x == nil {
		_ = hs.Shutdown(sh)
	}
	gs.GracefulStop()
	_ = lis.Close()
	_ = m
}

func deadlineInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		c, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return h(c, req)
	}
}

type auditIdentity struct{ UserID, TraceID, RequestID string }
type auditIdentityKey struct{}

func withAuditIdentity(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, auditIdentityKey{}, auditIdentity{UserID: userID, TraceID: uuid.NewString(), RequestID: uuid.NewString()})
}
func trustedAuditIdentity(ctx context.Context) auditIdentity {
	if v, ok := ctx.Value(auditIdentityKey{}).(auditIdentity); ok {
		return v
	}
	return auditIdentity{TraceID: uuid.NewString(), RequestID: uuid.NewString()}
}
func metricsInterceptor(m *observability.Metrics, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		start := time.Now()
		identity := trustedAuditIdentity(ctx)
		ctx = context.WithValue(ctx, auditIdentityKey{}, identity)
		m.RequestsTotal.WithLabelValues("checkin-service", info.FullMethod).Inc()
		out, err := h(ctx, req)
		m.DurationSeconds.WithLabelValues("checkin-service", info.FullMethod).Observe(time.Since(start).Seconds())
		if err != nil {
			m.ErrorsTotal.WithLabelValues("checkin-service", info.FullMethod).Inc()
		}
		logger.InfoContext(ctx, "request completed", "level", "info", "trace_id", identity.TraceID, "request_id", identity.RequestID, "user_id", identity.UserID, "method", info.FullMethod, "error", err)
		return out, err
	}
}
func pingDB(ctx context.Context, db interface{ PingContext(context.Context) error }) bool {
	c, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return db.PingContext(c) == nil
}
func pingRedis(ctx context.Context, r interface {
	Ping(context.Context) *redis.StatusCmd
}) bool {
	c, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return r.Ping(c).Err() == nil
}
