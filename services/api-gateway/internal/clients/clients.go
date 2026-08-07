package clients

import (
	"context"
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
	planv1 "github.com/example/fitness-checkin/proto/gen/plan/v1"
	profilev1 "github.com/example/fitness-checkin/proto/gen/profile/v1"
	statisticsv1 "github.com/example/fitness-checkin/proto/gen/statistics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

type Clients struct {
	Auth       authv1.AuthServiceClient
	Plan       planv1.PlanServiceClient
	Checkin    checkinv1.CheckinServiceClient
	Profile    profilev1.ProfileServiceClient
	Statistics statisticsv1.StatisticsServiceClient
	conns      []*grpc.ClientConn
}

func Dial(ctx context.Context, addresses map[string]string) (*Clients, error) {
	c := &Clients{}
	for name, addr := range addresses {
		conn, e := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if e != nil {
			c.Close()
			return nil, e
		}
		c.conns = append(c.conns, conn)
		switch name {
		case "auth":
			c.Auth = authv1.NewAuthServiceClient(conn)
		case "plan":
			c.Plan = planv1.NewPlanServiceClient(conn)
		case "checkin":
			c.Checkin = checkinv1.NewCheckinServiceClient(conn)
		case "profile":
			c.Profile = profilev1.NewProfileServiceClient(conn)
		case "statistics":
			c.Statistics = statisticsv1.NewStatisticsServiceClient(conn)
		}
	}
	return c, nil
}
func (c *Clients) Close() {
	for _, conn := range c.conns {
		_ = conn.Close()
	}
}
func CallContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}
