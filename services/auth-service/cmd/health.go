package main

import (
	"context"
	"github.com/example/fitness-checkin/pkg/servicehealth"
	"log/slog"
	"net"
)

type grpcServer interface{ Serve(net.Listener) error }

func serveGRPC(ctx context.Context, server grpcServer, lis net.Listener, health *servicehealth.Server, logger *slog.Logger, cancel context.CancelFunc) {
	health.SetServing(true)
	if err := server.Serve(lis); err != nil && ctx.Err() == nil {
		health.SetServing(false)
		logger.Error("grpc stopped", "error", err)
		cancel()
	}
	health.SetServing(false)
}
