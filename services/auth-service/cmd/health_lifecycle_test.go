package main

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/servicehealth"
	"log/slog"
	"net"
	"testing"
)

type serveStub struct {
	err      error
	observed chan bool
	health   *servicehealth.Server
}

func (s serveStub) Serve(net.Listener) error {
	s.observed <- s.health.Serving(context.Background())
	return s.err
}
func TestServeGRPCTransitionsHealth(t *testing.T) {
	h := servicehealth.New()
	observed := make(chan bool, 1)
	cancelled := false
	serveGRPC(context.Background(), serveStub{err: errors.New("boom"), observed: observed, health: h}, nil, h, slog.Default(), func() { cancelled = true })
	if !<-observed {
		t.Fatal("not serving during Serve")
	}
	if h.Serving(t.Context()) || !cancelled {
		t.Fatal("health not reset on serve error")
	}
}
