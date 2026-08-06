package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/pkg/storage"
	planv1 "github.com/example/fitness-checkin/proto/gen/plan/v1"
	plangrpc "github.com/example/fitness-checkin/services/plan-service/internal/grpc"
	"github.com/example/fitness-checkin/services/plan-service/internal/repository"
	"github.com/example/fitness-checkin/services/plan-service/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := observability.NewLogger("plan-service", nil)
	slog.SetDefault(logger)
	cfg, e := config.Load("plan-service")
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
	reg := observability.NewRegistry()
	m := observability.NewMetrics(reg)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(deadline(5*time.Second), metrics(m)))
	planv1.RegisterPlanServiceServer(gs, plangrpc.NewServer(service.New(repository.GORM{DB: db, Schema: repository.DefaultSchema})))
	lis, e := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if e != nil {
		logger.Error("listen failed", "error", e)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		sql, e := db.DB()
		if e != nil || sql.PingContext(r.Context()) != nil {
			http.Error(w, "not ready", 503)
			return
		}
		w.WriteHeader(200)
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if e := gs.Serve(lis); e != nil {
			logger.Error("grpc stopped", "error", e)
			cancel()
		}
	}()
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
	if sql, e := db.DB(); e == nil {
		_ = sql.Close()
	}
}
func deadline(d time.Duration) grpc.UnaryServerInterceptor {
	return func(c context.Context, r any, i *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		if _, ok := c.Deadline(); ok {
			return h(c, r)
		}
		x, cancel := context.WithTimeout(c, d)
		defer cancel()
		return h(x, r)
	}
}
func metrics(m *observability.Metrics) grpc.UnaryServerInterceptor {
	return func(c context.Context, r any, i *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		start := time.Now()
		m.RequestsTotal.WithLabelValues("plan-service", i.FullMethod).Inc()
		out, e := h(c, r)
		m.DurationSeconds.WithLabelValues("plan-service", i.FullMethod).Observe(time.Since(start).Seconds())
		if e != nil {
			m.ErrorsTotal.WithLabelValues("plan-service", i.FullMethod).Inc()
		}
		return out, e
	}
}

var _ = keepalive.ServerParameters{}
