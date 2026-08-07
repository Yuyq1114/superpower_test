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
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	authgrpc "github.com/example/fitness-checkin/services/auth-service/internal/grpc"
	"github.com/example/fitness-checkin/services/auth-service/internal/repository"
	"github.com/example/fitness-checkin/services/auth-service/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

func main() {
	logger := observability.NewLogger("auth-service", nil)
	slog.SetDefault(logger)
	cfg, err := config.Load("auth-service")
	if err != nil {
		logger.Error("startup failed", "error", err)
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	connectCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	db, err := storage.OpenPostgres(connectCtx, cfg)
	if err != nil {
		logger.Error("database failed", "error", err)
		return
	}
	if err = repository.Migrate(connectCtx, db); err != nil {
		logger.Error("migration failed", "error", err)
		return
	}
	uow := repository.GORMUnitOfWork{DB: db}
	svc := service.NewAuthService(repository.GORMUser{DB: db, Schema: repository.DefaultSchema}, repository.GORMRefreshToken{DB: db, Schema: repository.DefaultSchema}, uow, service.NewTokenManager([]byte(cfg.JWTSecret), 15*time.Minute, 30*24*time.Hour))
	reg := observability.NewRegistry()
	metrics := observability.NewMetrics(reg)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(defaultDeadlineInterceptor(5*time.Second), requestLogger(logger), metricsInterceptor(metrics)), grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute, Time: 2 * time.Hour, Timeout: 20 * time.Second}), grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 30 * time.Second, PermitWithoutStream: true}))
	authv1.RegisterAuthServiceServer(gs, authgrpc.NewServer(svc))
	sqlDB, _ := db.DB()
	health := servicehealth.New(func(c context.Context) error {
		x, cancel := context.WithTimeout(c, 500*time.Millisecond)
		defer cancel()
		return sqlDB.PingContext(x)
	})
	healthv1.RegisterHealthServer(gs, health)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		logger.Error("listen failed", "error", err)
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if sqlDB == nil || !health.Serving(r.Context()) {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go serveGRPC(ctx, gs, lis, health, logger, cancel)

	go func() {
		if e := hs.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			logger.Error("http stopped", "error", e)
			cancel()
		}
	}()
	<-ctx.Done()
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_ = hs.Shutdown(shutdown)
	gs.GracefulStop()
	_ = lis.Close()
	if c, e := db.DB(); e == nil {
		_ = c.Close()
	}
}
