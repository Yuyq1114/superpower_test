package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/pkg/storage"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
	planv1 "github.com/example/fitness-checkin/proto/gen/plan/v1"
	"github.com/example/fitness-checkin/services/checkin-service/internal/events"
	checkingrpc "github.com/example/fitness-checkin/services/checkin-service/internal/grpc"
	"github.com/example/fitness-checkin/services/checkin-service/internal/repository"
	"github.com/example/fitness-checkin/services/checkin-service/internal/service"
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

type planChecker struct{ client planv1.PlanServiceClient }

func (p planChecker) CheckWorkoutItem(ctx context.Context, u, i string) error {
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
	gs := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute, Time: 2 * time.Minute, Timeout: 20 * time.Second}), grpc.MaxRecvMsgSize(4<<20), grpc.MaxSendMsgSize(4<<20))
	checkinv1.RegisterCheckinServiceServer(gs, checkingrpc.NewServer(service.New(repository.GORM{DB: db, Schema: repository.DefaultSchema}, planChecker{client: planv1.NewPlanServiceClient(pc)})))
	pub := &events.Publisher{Repo: repository.GORM{DB: db, Schema: repository.DefaultSchema}, Redis: red}
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
		if x != nil || sql.PingContext(r.Context()) != nil || red.Ping(r.Context()).Err() != nil {
			http.Error(w, "not ready", 503)
			return
		}
		w.WriteHeader(200)
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	go gs.Serve(lis)
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
