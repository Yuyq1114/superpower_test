package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/pkg/storage"
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	authgrpc "github.com/example/fitness-checkin/services/auth-service/internal/grpc"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"github.com/example/fitness-checkin/services/auth-service/internal/repository"
	"github.com/example/fitness-checkin/services/auth-service/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, e := config.Load("auth-service")
	if e != nil {
		panic(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	connectCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	db, e := storage.OpenPostgres(connectCtx, cfg)
	if e != nil {
		panic(e)
	}
	if e = db.WithContext(connectCtx).Exec("CREATE TABLE IF NOT EXISTS auth_schema.users (id text primary key,email text not null unique,password_hash text not null,created_at timestamptz not null,updated_at timestamptz not null)").Error; e != nil {
		panic(e)
	}
	if e = db.WithContext(connectCtx).Exec("CREATE TABLE IF NOT EXISTS auth_schema.refresh_tokens (id text primary key,user_id text not null,token_hash text not null unique,expires_at timestamptz not null,revoked_at timestamptz,created_at timestamptz not null)").Error; e != nil {
		panic(e)
	}
	svc := service.NewAuthService(repository.GORMUser{DB: db}, repository.GORMRefreshToken{DB: db}, service.NewTokenManager([]byte(cfg.JWTSecret), 15*time.Minute, 30*24*time.Hour))
	gs := grpc.NewServer()
	authv1.RegisterAuthServiceServer(gs, authgrpc.NewServer(svc))
	lis, e := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if e != nil {
		panic(e)
	}
	reg := observability.NewRegistry()
	observability.NewMetrics(reg)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		c, e := db.DB()
		if e != nil || c.PingContext(r.Context()) != nil {
			http.Error(w, "not ready", 503)
			return
		}
		w.WriteHeader(200)
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: mux}
	go gs.Serve(lis)
	go hs.ListenAndServe()
	<-ctx.Done()
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	hs.Shutdown(shutdown)
	gs.GracefulStop()
	lis.Close()
	if c, e := db.DB(); e == nil {
		c.Close()
	}
}

var _ = errors.New
var _ = model.User{}
