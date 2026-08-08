package clients

import (
	"context"
	"google.golang.org/grpc"
	"net"
	"testing"
	"time"
)

func TestDialConnectsAndCloseShutsConnection(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	go server.Serve(lis)
	t.Cleanup(func() { server.Stop(); lis.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := Dial(ctx, map[string]string{"auth": lis.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.conns) != 1 || c.Auth == nil {
		t.Fatalf("clients=%+v conns=%d", c, len(c.conns))
	}
	conn := c.conns[0]
	c.Close()
	deadline := time.Now().Add(time.Second)
	for conn.GetState().String() != "SHUTDOWN" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if conn.GetState().String() != "SHUTDOWN" {
		t.Fatalf("state=%s", conn.GetState())
	}
}
