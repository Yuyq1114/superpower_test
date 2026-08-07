package http

import (
	"context"
	"errors"
	"log/slog"
	nethttp "net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/example/fitness-checkin/pkg/apperror"
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
	planv1 "github.com/example/fitness-checkin/proto/gen/plan/v1"
	profilev1 "github.com/example/fitness-checkin/proto/gen/profile/v1"
	statisticsv1 "github.com/example/fitness-checkin/proto/gen/statistics/v1"
	"github.com/example/fitness-checkin/services/api-gateway/internal/auth"
	"github.com/example/fitness-checkin/services/api-gateway/internal/clients"
	"github.com/example/fitness-checkin/services/api-gateway/internal/mapper"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

type Dependencies struct {
	Clients    *clients.Clients
	Auth       authv1.AuthServiceClient
	Plan       planv1.PlanServiceClient
	Checkin    checkinv1.CheckinServiceClient
	Profile    profilev1.ProfileServiceClient
	Statistics statisticsv1.StatisticsServiceClient
	JWTSecret  string
	Logger     *slog.Logger
	Ready      func(context.Context) error
}
type handler struct{ d *Dependencies }

var validID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func NewRouter(d *Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if d == nil {
		d = &Dependencies{}
	}
	if d.Clients != nil {
		d.Auth = d.Clients.Auth
		d.Plan = d.Clients.Plan
		d.Checkin = d.Clients.Checkin
		d.Profile = d.Clients.Profile
		d.Statistics = d.Clients.Statistics
	}
	h := &handler{d: d}
	r.Use(gin.Recovery(), h.correlation(), bodyLimit(1<<20), h.logging())
	r.GET("/healthz", func(c *gin.Context) { c.Status(200) })
	r.GET("/readyz", h.ready)
	v := r.Group("/api/v1")
	a := v.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/login", h.login)
	a.POST("/refresh", h.refresh)
	protected := v.Group("")
	protected.Use(h.authenticate())
	protected.POST("/auth/logout", h.logout)
	protected.POST("/plans", h.createPlan)
	protected.GET("/plans", h.listPlans)
	protected.GET("/plans/:plan_id", h.getPlan)
	protected.PUT("/plans/:plan_id", h.updatePlan)
	protected.DELETE("/plans/:plan_id", h.deletePlan)
	protected.POST("/plans/:plan_id/days", h.addDay)
	protected.GET("/plans/:plan_id/days", h.listDays)
	protected.GET("/plans/:plan_id/days/:day_id", h.getDay)
	protected.PUT("/plans/:plan_id/days/:day_id", h.updateDay)
	protected.DELETE("/plans/:plan_id/days/:day_id", h.deleteDay)
	protected.POST("/workout-days/:day_id/items", h.addItem)
	protected.GET("/workout-days/:day_id/items", h.listItems)
	protected.GET("/workout-days/:day_id/items/:item_id", h.getItem)
	protected.PUT("/workout-days/:day_id/items/:item_id", h.updateItem)
	protected.DELETE("/workout-days/:day_id/items/:item_id", h.deleteItem)
	protected.POST("/checkins", h.complete)
	protected.GET("/checkins", h.history)
	protected.GET("/checkins/streak", h.history)
	protected.POST("/body-metrics", h.recordMetric)
	protected.GET("/body-metrics", h.listMetrics)
	protected.GET("/statistics/summary", h.summary)
	return r
}
func (h *handler) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, err := auth.Authenticate(c.Request.Context(), c.GetHeader("Authorization"), h.d.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(mapper.HTTPStatus(err), mapper.Error(err, requestID(c)))
			return
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
func (h *handler) correlation() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if !validID.MatchString(rid) {
			rid = uuid.NewString()
		}
		tid := c.GetHeader("X-Trace-ID")
		if !validID.MatchString(tid) {
			tid = uuid.NewString()
		}
		c.Set("request_id", rid)
		c.Set("trace_id", tid)
		c.Header("X-Request-ID", rid)
		c.Header("X-Trace-ID", tid)
		c.Next()
	}
}
func bodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > n {
			c.AbortWithStatusJSON(nethttp.StatusRequestEntityTooLarge, mapper.Error(&nethttp.MaxBytesError{Limit: n}, requestID(c)))
			return
		}
		c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}
func (h *handler) logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if h.d.Logger != nil {
			h.d.Logger.Info("request completed", "request_id", requestID(c), "trace_id", c.GetString("trace_id"), "user_id", auth.UserID(c.Request.Context()), "method", c.Request.Method, "path", c.FullPath(), "status", c.Writer.Status(), "duration", time.Since(start))
		}
	}
}
func requestID(c *gin.Context) string { return c.GetString("request_id") }
func (h *handler) ready(c *gin.Context) {
	if h.d.Ready != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if err := h.d.Ready(ctx); err != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
	}
	c.Status(200)
}
func (h *handler) publicContext(c *gin.Context) (context.Context, context.CancelFunc) {
	ctx := metadata.NewOutgoingContext(c.Request.Context(), metadata.Pairs("x-request-id", requestID(c), "x-trace-id", c.GetString("trace_id")))
	return clients.CallContext(ctx)
}
func (h *handler) grpcContext(c *gin.Context) (context.Context, context.CancelFunc) {
	ctx := metadata.NewOutgoingContext(c.Request.Context(), metadata.Pairs("authorization", c.GetHeader("Authorization"), "x-request-id", requestID(c), "x-trace-id", c.GetString("trace_id")))
	return clients.CallContext(ctx)
}
func (h *handler) fail(c *gin.Context, e error) {
	c.JSON(mapper.HTTPStatus(e), mapper.Error(e, requestID(c)))
}
func bind(c *gin.Context, v any) error {
	if e := c.ShouldBindJSON(v); e != nil {
		var max *nethttp.MaxBytesError
		if errors.As(e, &max) {
			return max
		}
		if strings.Contains(e.Error(), "request body too large") {
			return &nethttp.MaxBytesError{Limit: 1 << 20}
		}
		return apperror.InvalidArgument("invalid JSON request")
	}
	return nil
}
func uid(c *gin.Context) string { return auth.UserID(c.Request.Context()) }
func page(c *gin.Context) *planv1.PageRequest {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	z, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return &planv1.PageRequest{Page: int32(p), PageSize: int32(z)}
}
func key(c *gin.Context) string { return c.GetHeader("Idempotency-Key") }

func (h *handler) register(c *gin.Context) {
	var r authv1.RegisterRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	ctx, x := h.publicContext(c)
	defer x()
	o, e := h.d.Auth.Register(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(201, o)
}
func (h *handler) login(c *gin.Context) {
	var r authv1.LoginRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	ctx, x := h.publicContext(c)
	defer x()
	o, e := h.d.Auth.Login(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) refresh(c *gin.Context) {
	var r authv1.RefreshRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	ctx, x := h.publicContext(c)
	defer x()
	o, e := h.d.Auth.Refresh(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) logout(c *gin.Context) {
	var r authv1.LogoutRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	ctx, x := h.grpcContext(c)
	defer x()
	_, e := h.d.Auth.Logout(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.Status(204)
}
func (h *handler) createPlan(c *gin.Context) {
	var r planv1.CreatePlanRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	r.UserId = uid(c)
	r.IdempotencyKey = key(c)
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.CreatePlan(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(201, o)
}
func (h *handler) listPlans(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.ListPlans(ctx, &planv1.ListPlansRequest{UserId: uid(c), Page: page(c)})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) getPlan(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.GetPlan(ctx, &planv1.GetPlanRequest{UserId: uid(c), PlanId: c.Param("plan_id")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) updatePlan(c *gin.Context) {
	var r planv1.UpdatePlanRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	r.UserId = uid(c)
	r.PlanId = c.Param("plan_id")
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.UpdatePlan(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) deletePlan(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	_, e := h.d.Plan.DeletePlan(ctx, &planv1.DeletePlanRequest{UserId: uid(c), PlanId: c.Param("plan_id")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.Status(204)
}
func (h *handler) addDay(c *gin.Context) {
	var r planv1.AddWorkoutDayRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	r.UserId = uid(c)
	r.PlanId = c.Param("plan_id")
	r.IdempotencyKey = key(c)
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.AddWorkoutDay(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(201, o)
}
func (h *handler) listDays(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.ListWorkoutDays(ctx, &planv1.ListWorkoutDaysRequest{UserId: uid(c), PlanId: c.Param("plan_id"), Page: page(c)})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) getDay(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.GetWorkoutDay(ctx, &planv1.GetWorkoutDayRequest{UserId: uid(c), PlanId: c.Param("plan_id"), WorkoutDayId: c.Param("day_id")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) updateDay(c *gin.Context) {
	var r planv1.UpdateWorkoutDayRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	r.UserId = uid(c)
	r.PlanId = c.Param("plan_id")
	r.WorkoutDayId = c.Param("day_id")
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.UpdateWorkoutDay(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) deleteDay(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	_, e := h.d.Plan.DeleteWorkoutDay(ctx, &planv1.DeleteWorkoutDayRequest{UserId: uid(c), PlanId: c.Param("plan_id"), WorkoutDayId: c.Param("day_id")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.Status(204)
}
func (h *handler) addItem(c *gin.Context) {
	var body struct {
		Item planv1.WorkoutItem `json:"item"`
	}
	if e := bind(c, &body); e != nil {
		h.fail(c, e)
		return
	}
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.AddWorkoutItem(ctx, &planv1.AddWorkoutItemRequest{UserId: uid(c), WorkoutDayId: c.Param("day_id"), Item: &body.Item, IdempotencyKey: key(c)})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(201, o)
}
func (h *handler) listItems(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	p := page(c)
	o, e := h.d.Plan.ListWorkoutItems(ctx, &planv1.ListWorkoutItemsRequest{UserId: uid(c), WorkoutDayId: c.Param("day_id"), Page: p})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) getItem(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.GetWorkoutItem(ctx, &planv1.GetWorkoutItemRequest{UserId: uid(c), WorkoutDayId: c.Param("day_id"), WorkoutItemId: c.Param("item_id")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) updateItem(c *gin.Context) {
	var body struct {
		Item planv1.WorkoutItem `json:"item"`
	}
	if e := bind(c, &body); e != nil {
		h.fail(c, e)
		return
	}
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Plan.UpdateWorkoutItem(ctx, &planv1.UpdateWorkoutItemRequest{UserId: uid(c), WorkoutDayId: c.Param("day_id"), WorkoutItemId: c.Param("item_id"), Item: &body.Item})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) deleteItem(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	_, e := h.d.Plan.DeleteWorkoutItem(ctx, &planv1.DeleteWorkoutItemRequest{UserId: uid(c), WorkoutDayId: c.Param("day_id"), WorkoutItemId: c.Param("item_id")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.Status(204)
}
func (h *handler) complete(c *gin.Context) {
	var r checkinv1.CompleteRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	r.UserId = uid(c)
	r.IdempotencyKey = key(c)
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Checkin.Complete(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(201, o)
}
func (h *handler) history(c *gin.Context) {
	p := page(c)
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Checkin.ListHistory(ctx, &checkinv1.ListHistoryRequest{UserId: uid(c), From: c.Query("from"), To: c.Query("to"), Page: &checkinv1.PageRequest{Page: p.Page, PageSize: p.PageSize}})
	if e != nil {
		h.fail(c, e)
		return
	}
	if strings.HasSuffix(c.FullPath(), "/streak") {
		c.JSON(200, gin.H{"streak": o.Streak})
		return
	}
	c.JSON(200, o)
}
func (h *handler) recordMetric(c *gin.Context) {
	var r profilev1.RecordMetricRequest
	if e := bind(c, &r); e != nil {
		h.fail(c, e)
		return
	}
	r.UserId = uid(c)
	r.IdempotencyKey = key(c)
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Profile.RecordMetric(ctx, &r)
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(201, o)
}
func (h *handler) listMetrics(c *gin.Context) {
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Profile.ListMetrics(ctx, &profilev1.ListMetricsRequest{UserId: uid(c), MetricType: c.Query("metric_type"), From: c.Query("from"), To: c.Query("to")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
func (h *handler) summary(c *gin.Context) {
	period := statisticsv1.Period_PERIOD_WEEK
	switch strings.ToLower(strings.TrimSpace(c.Query("period"))) {
	case "", "week":
	case "month":
		period = statisticsv1.Period_PERIOD_MONTH
	default:
		h.fail(c, apperror.InvalidArgument("period must be week or month"))
		return
	}
	ctx, x := h.grpcContext(c)
	defer x()
	o, e := h.d.Statistics.GetSummary(ctx, &statisticsv1.GetSummaryRequest{UserId: uid(c), Period: period, Start: c.Query("start"), End: c.Query("end")})
	if e != nil {
		h.fail(c, e)
		return
	}
	c.JSON(200, o)
}
