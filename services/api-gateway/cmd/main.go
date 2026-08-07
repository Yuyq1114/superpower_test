package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/example/fitness-checkin/pkg/observability"
	gatewayclients "github.com/example/fitness-checkin/services/api-gateway/internal/clients"
	gatewayhttp "github.com/example/fitness-checkin/services/api-gateway/internal/http"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type gatewayConfig struct {
	Port      int
	Secret    string
	Addresses map[string]string
}

func load() (gatewayConfig, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return gatewayConfig{}, fmt.Errorf("JWT_SECRET is required")
	}
	p, _ := strconv.Atoi(os.Getenv("HTTP_PORT"))
	if p <= 0 {
		p = 8080
	}
	a := map[string]string{"auth": env("AUTH_GRPC_ADDR", "localhost:9091"), "plan": env("PLAN_GRPC_ADDR", "localhost:9092"), "checkin": env("CHECKIN_GRPC_ADDR", "localhost:9093"), "profile": env("PROFILE_GRPC_ADDR", "localhost:9094"), "statistics": env("STATISTICS_GRPC_ADDR", "localhost:9095")}
	return gatewayConfig{p, secret, a}, nil
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	if err := run(); err != nil {
		slog.Error("api gateway stopped", "error", err)
		os.Exit(1)
	}
}
func productionDependencies(cs *gatewayclients.Clients, secret string, logger *slog.Logger) *gatewayhttp.Dependencies {
	return &gatewayhttp.Dependencies{Clients: cs, JWTSecret: secret, Logger: logger, Ready: cs.Ready}
}
func run() error {
	logger := observability.NewLogger("api-gateway", nil)
	slog.SetDefault(logger)
	cfg, e := load()
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	cs, e := gatewayclients.Dial(startup, cfg.Addresses)
	if e != nil {
		return fmt.Errorf("connect grpc services: %w", e)
	}
	defer cs.Close()
	reg := observability.NewRegistry()
	_ = observability.NewMetrics(reg)
	router := gatewayhttp.NewRouter(productionDependencies(cs, cfg.Secret, logger))
	router.GET("/metrics", func(c *gin.Context) { promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(c.Writer, c.Request) })
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	fail := make(chan error, 1)
	go func() {
		if x := srv.ListenAndServe(); x != nil && !errors.Is(x, http.ErrServerClosed) {
			fail <- x
		}
	}()
	select {
	case e = <-fail:
		cancel()
	case <-ctx.Done():
		e = nil
	}
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = srv.Shutdown(shutdown)
	return e
}
