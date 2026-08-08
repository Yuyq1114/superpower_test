package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/pkg/servicehealth"
	"github.com/example/fitness-checkin/pkg/storage"
	profilev1 "github.com/example/fitness-checkin/proto/gen/profile/v1"
	profilgrpc "github.com/example/fitness-checkin/services/profile-service/internal/grpc"
	"github.com/example/fitness-checkin/services/profile-service/internal/identity"
	"github.com/example/fitness-checkin/services/profile-service/internal/repository"
	"github.com/example/fitness-checkin/services/profile-service/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

type servingState struct{ serving atomic.Bool }

func (s *servingState) Set(v bool)  { s.serving.Store(v) }
func (s *servingState) Ready() bool { return s.serving.Load() }
func main() {
	logger := observability.NewLogger("profile-service", nil)
	slog.SetDefault(logger)
	cfg, e := config.Load("profile-service")
	if e != nil {
		logger.Error("startup failed", "error", e)
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cc, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	db, e := storage.OpenPostgres(cc, cfg)
	if e != nil {
		logger.Error("database failed", "error", e)
		return
	}
	if e = repository.Migrate(cc, db); e != nil {
		logger.Error("migration failed", "error", e)
		return
	}
	reg := observability.NewRegistry()
	m := observability.NewMetrics(reg)
	state := &servingState{}
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(identity.UnaryServerInterceptor(cfg.JWTSecret), deadlineInterceptor(5*time.Second), metricsInterceptor(m, logger)), grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute, Time: 2 * time.Minute, Timeout: 20 * time.Second}))
	profilev1.RegisterProfileServiceServer(gs, profilgrpc.NewServer(service.New(repository.GORM{DB: db, Schema: repository.DefaultSchema})))
	sqlDB, _ := db.DB()
	health := servicehealth.New(func(c context.Context) error {
		if !state.Ready() {
			return errors.New("grpc not serving")
		}
		x, cancel := context.WithTimeout(c, 500*time.Millisecond)
		defer cancel()
		return sqlDB.PingContext(x)
	})
	healthv1.RegisterHealthServer(gs, health)
	lis, e := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if e != nil {
		logger.Error("listen failed", "error", e)
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		sql, x := db.DB()
		if x != nil || !state.Ready() || !pingDB(r.Context(), sql) {
			http.Error(w, "not ready", 503)
			return
		}
		w.WriteHeader(200)
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	health.SetServing(true)
	go serveGRPC(ctx, gs, lis, state, logger, cancel)
	go func() {
		if x := hs.ListenAndServe(); x != nil && !errors.Is(x, http.ErrServerClosed) {
			logger.Error("http stopped", "error", x)
			cancel()
		}
	}()
	<-ctx.Done()
	state.Set(false)
	sh, x := context.WithTimeout(context.Background(), 5*time.Second)
	if x == nil {
		_ = hs.Shutdown(sh)
	}
	gs.GracefulStop()
	_ = lis.Close()
}
func serveGRPC(ctx context.Context, server interface{ Serve(net.Listener) error }, lis net.Listener, state *servingState, logger *slog.Logger, cancel context.CancelFunc) {
	state.Set(true)
	if e := server.Serve(lis); e != nil && ctx.Err() == nil {
		state.Set(false)
		logger.Error("grpc stopped", "error", e)
		cancel()
	}
	state.Set(false)
}
func pingDB(ctx context.Context, db interface{ PingContext(context.Context) error }) bool {
	c, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return db.PingContext(c) == nil
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
		start := time.Now()
		user, trace, request, _ := identity.FromContext(ctx)
		m.RequestsTotal.WithLabelValues("profile-service", info.FullMethod).Inc()
		out, e := h(ctx, req)
		m.DurationSeconds.WithLabelValues("profile-service", info.FullMethod).Observe(time.Since(start).Seconds())
		if e != nil {
			m.ErrorsTotal.WithLabelValues("profile-service", info.FullMethod).Inc()
		}
		logger.InfoContext(ctx, "request completed", "trace_id", trace, "request_id", request, "user_id", user, "method", info.FullMethod, "error", e)
		return out, e
	}
}
