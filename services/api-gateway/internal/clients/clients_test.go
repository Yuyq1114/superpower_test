package clients

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	health "google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

func healthClient(t *testing.T, status healthv1.HealthCheckResponse_ServingStatus) healthv1.HealthClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	hs := health.NewServer()
	hs.SetServingStatus("", status)
	healthv1.RegisterHealthServer(s, hs)
	go s.Serve(lis)
	t.Cleanup(func() { s.Stop(); lis.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "buf", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return healthv1.NewHealthClient(conn)
}
func TestReadyChecksEveryRequiredService(t *testing.T) {
	good := healthClient(t, healthv1.HealthCheckResponse_SERVING)
	c := &Clients{health: map[string]healthv1.HealthClient{"auth": good, "plan": good, "checkin": good, "profile": good, "statistics": good}}
	if err := c.Ready(t.Context()); err != nil {
		t.Fatal(err)
	}
	c.health["profile"] = healthClient(t, healthv1.HealthCheckResponse_NOT_SERVING)
	if err := c.Ready(t.Context()); err == nil {
		t.Fatal("expected non-serving dependency to fail readiness")
	}
}
func TestReadyRejectsUnimplementedHealth(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	go s.Serve(lis)
	t.Cleanup(func() { s.Stop(); lis.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "buf", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	h := healthv1.NewHealthClient(conn)
	c := &Clients{health: map[string]healthv1.HealthClient{}}
	for _, name := range requiredServices {
		c.health[name] = h
	}
	if err := c.Ready(context.Background()); err == nil {
		t.Fatal("unimplemented health must not be ready")
	}
}
