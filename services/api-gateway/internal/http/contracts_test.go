package http

import (
	"context"
	"encoding/json"
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
	planv1 "github.com/example/fitness-checkin/proto/gen/plan/v1"
	profilev1 "github.com/example/fitness-checkin/proto/gen/profile/v1"
	statisticsv1 "github.com/example/fitness-checkin/proto/gen/statistics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type authStub struct {
	authv1.AuthServiceClient
	login   func(context.Context, *authv1.LoginRequest) (*authv1.AuthResponse, error)
	refresh func(context.Context, *authv1.RefreshRequest) (*authv1.RefreshResponse, error)
	logout  func(context.Context, *authv1.LogoutRequest) (*authv1.Empty, error)
}

func (s authStub) Login(c context.Context, r *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return s.login(c, r)
}

func (s authStub) Refresh(c context.Context, r *authv1.RefreshRequest, _ ...grpc.CallOption) (*authv1.RefreshResponse, error) {
	return s.refresh(c, r)
}

func (s authStub) Logout(c context.Context, r *authv1.LogoutRequest, _ ...grpc.CallOption) (*authv1.Empty, error) {
	return s.logout(c, r)
}

type planStub struct {
	planv1.PlanServiceClient
	create  func(context.Context, *planv1.CreatePlanRequest) (*planv1.PlanResponse, error)
	addItem func(context.Context, *planv1.AddWorkoutItemRequest) (*planv1.WorkoutItemResponse, error)
}

func (s planStub) CreatePlan(c context.Context, r *planv1.CreatePlanRequest, _ ...grpc.CallOption) (*planv1.PlanResponse, error) {
	return s.create(c, r)
}
func (s planStub) AddWorkoutItem(c context.Context, r *planv1.AddWorkoutItemRequest, _ ...grpc.CallOption) (*planv1.WorkoutItemResponse, error) {
	return s.addItem(c, r)
}

type checkinStub struct {
	checkinv1.CheckinServiceClient
	complete func(context.Context, *checkinv1.CompleteRequest) (*checkinv1.CompleteResponse, error)
	history  func(context.Context, *checkinv1.ListHistoryRequest) (*checkinv1.ListHistoryResponse, error)
}

func (s checkinStub) Complete(c context.Context, r *checkinv1.CompleteRequest, _ ...grpc.CallOption) (*checkinv1.CompleteResponse, error) {
	return s.complete(c, r)
}
func (s checkinStub) ListHistory(c context.Context, r *checkinv1.ListHistoryRequest, _ ...grpc.CallOption) (*checkinv1.ListHistoryResponse, error) {
	return s.history(c, r)
}

type profileStub struct {
	profilev1.ProfileServiceClient
	record func(context.Context, *profilev1.RecordMetricRequest) (*profilev1.RecordMetricResponse, error)
	list   func(context.Context, *profilev1.ListMetricsRequest) (*profilev1.ListMetricsResponse, error)
}

func (s profileStub) RecordMetric(c context.Context, r *profilev1.RecordMetricRequest, _ ...grpc.CallOption) (*profilev1.RecordMetricResponse, error) {
	return s.record(c, r)
}
func (s profileStub) ListMetrics(c context.Context, r *profilev1.ListMetricsRequest, _ ...grpc.CallOption) (*profilev1.ListMetricsResponse, error) {
	return s.list(c, r)
}

type statsStub struct {
	statisticsv1.StatisticsServiceClient
	get func(context.Context, *statisticsv1.GetSummaryRequest) (*statisticsv1.GetSummaryResponse, error)
}

func (s statsStub) GetSummary(c context.Context, r *statisticsv1.GetSummaryRequest, _ ...grpc.CallOption) (*statisticsv1.GetSummaryResponse, error) {
	return s.get(c, r)
}
func md(t *testing.T, c context.Context) metadata.MD {
	t.Helper()
	m, ok := metadata.FromOutgoingContext(c)
	if !ok {
		t.Fatal("missing metadata")
	}
	return m
}
func call(t *testing.T, r nethttp.Handler, method, path, body string, protected bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req-1")
	req.Header.Set("X-Trace-ID", "trace-1")
	if protected {
		req.Header.Set("Authorization", "Bearer "+token(t, "secret", time.Now().Add(time.Hour)))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestPublicAuthPropagatesCorrelationWithoutAuthorization(t *testing.T) {
	a := authStub{login: func(c context.Context, r *authv1.LoginRequest) (*authv1.AuthResponse, error) {
		m := md(t, c)
		if len(m.Get("authorization")) != 0 || m.Get("x-request-id")[0] != "req-1" || m.Get("x-trace-id")[0] != "trace-1" {
			t.Fatalf("metadata=%v", m)
		}
		return &authv1.AuthResponse{User: &authv1.User{Id: "u"}}, nil
	}}
	w := call(t, NewRouter(&Dependencies{Auth: a}), "POST", "/api/v1/auth/login", `{"email":"a@b.com","password":"secret"}`, false)
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
func TestCreatePlanUsesTrustedIdentityAndNoTokenMetadata(t *testing.T) {
	p := planStub{create: func(c context.Context, r *planv1.CreatePlanRequest) (*planv1.PlanResponse, error) {
		m := md(t, c)
		if r.UserId != "trusted-user" || len(m.Get("authorization")) != 1 || !strings.HasPrefix(m.Get("authorization")[0], "Bearer ") {
			t.Fatalf("request=%+v md=%v", r, m)
		}
		return &planv1.PlanResponse{Plan: &planv1.Plan{Id: "p1", UserId: r.UserId, Name: r.Name}}, nil
	}}
	w := call(t, NewRouter(&Dependencies{JWTSecret: "secret", Plan: p}), "POST", "/api/v1/plans", `{"user_id":"attacker","name":"Strength"}`, true)
	if w.Code != 201 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
func TestWorkoutItemUsesNestedItemDTO(t *testing.T) {
	p := planStub{addItem: func(_ context.Context, r *planv1.AddWorkoutItemRequest) (*planv1.WorkoutItemResponse, error) {
		if r.UserId != "trusted-user" || r.WorkoutDayId != "d1" || r.Item.Name != "Squat" || r.Item.Sets != 3 || r.Item.Repetitions != 5 || r.Item.Weight != 80 || r.Item.DurationSeconds != 60 {
			t.Fatalf("request=%+v item=%+v", r, r.Item)
		}
		return &planv1.WorkoutItemResponse{Item: r.Item}, nil
	}}
	w := call(t, NewRouter(&Dependencies{JWTSecret: "secret", Plan: p}), "POST", "/api/v1/workout-days/d1/items", `{"item":{"name":"Squat","sets":3,"repetitions":5,"weight":80,"duration_seconds":60}}`, true)
	if w.Code != 201 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
func TestCheckinHandlersMapCompleteHistoryAndStreak(t *testing.T) {
	c := checkinStub{complete: func(_ context.Context, r *checkinv1.CompleteRequest) (*checkinv1.CompleteResponse, error) {
		if r.UserId != "trusted-user" || r.WorkoutItemId != "i1" {
			t.Fatalf("request=%+v", r)
		}
		return &checkinv1.CompleteResponse{Checkin: &checkinv1.Checkin{Id: "c1"}}, nil
	}, history: func(_ context.Context, r *checkinv1.ListHistoryRequest) (*checkinv1.ListHistoryResponse, error) {
		if r.UserId != "trusted-user" || r.From != "2026-08-01" {
			t.Fatalf("request=%+v", r)
		}
		return &checkinv1.ListHistoryResponse{Streak: 7}, nil
	}}
	r := NewRouter(&Dependencies{JWTSecret: "secret", Checkin: c})
	if w := call(t, r, "POST", "/api/v1/checkins", `{"user_id":"bad","workout_item_id":"i1","date":"2026-08-07"}`, true); w.Code != 201 {
		t.Fatalf("complete=%d %s", w.Code, w.Body.String())
	}
	if w := call(t, r, "GET", "/api/v1/checkins?from=2026-08-01&to=2026-08-07", "", true); w.Code != 200 {
		t.Fatalf("history=%d", w.Code)
	}
	w := call(t, r, "GET", "/api/v1/checkins/streak?from=2026-08-01&to=2026-08-07", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"streak":7`) {
		t.Fatalf("streak=%d %s", w.Code, w.Body.String())
	}
}
func TestBodyMetricHandlersMapRecordAndList(t *testing.T) {
	p := profileStub{record: func(_ context.Context, r *profilev1.RecordMetricRequest) (*profilev1.RecordMetricResponse, error) {
		if r.UserId != "trusted-user" || r.Value != 70.5 {
			t.Fatalf("request=%+v", r)
		}
		return &profilev1.RecordMetricResponse{Metric: &profilev1.Metric{Id: "m1"}}, nil
	}, list: func(_ context.Context, r *profilev1.ListMetricsRequest) (*profilev1.ListMetricsResponse, error) {
		if r.UserId != "trusted-user" || r.MetricType != "weight" {
			t.Fatalf("request=%+v", r)
		}
		return &profilev1.ListMetricsResponse{}, nil
	}}
	r := NewRouter(&Dependencies{JWTSecret: "secret", Profile: p})
	if w := call(t, r, "POST", "/api/v1/body-metrics", `{"user_id":"bad","metric_type":"weight","value":70.5,"unit":"kg"}`, true); w.Code != 201 {
		t.Fatalf("record=%d", w.Code)
	}
	if w := call(t, r, "GET", "/api/v1/body-metrics?metric_type=weight", "", true); w.Code != 200 {
		t.Fatalf("list=%d", w.Code)
	}
}
func TestStatisticsDefaultsWeekAndMapsSafeError(t *testing.T) {
	s := statsStub{get: func(_ context.Context, r *statisticsv1.GetSummaryRequest) (*statisticsv1.GetSummaryResponse, error) {
		if r.UserId != "trusted-user" || r.Period != statisticsv1.Period_PERIOD_WEEK {
			t.Fatalf("request=%+v", r)
		}
		return nil, status.Error(codes.Unavailable, "dial tcp password=secret")
	}}
	w := call(t, NewRouter(&Dependencies{JWTSecret: "secret", Statistics: s}), "GET", "/api/v1/statistics/summary", "", true)
	if w.Code != 503 || strings.Contains(w.Body.String(), "password") || strings.Contains(w.Body.String(), "dial tcp") {
		b, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("status=%d body=%s", w.Code, b)
	}
}
func TestResponseJSONCanDecode(t *testing.T) {
	var v map[string]any
	_ = json.Unmarshal([]byte(`{"ok":true}`), &v)
}

func TestLoginSetsRefreshCookieAndOmitsRefreshTokenFromBody(t *testing.T) {
	cfg := RefreshCookieConfig{Name: "fitness_refresh"}
	a := authStub{login: func(context.Context, *authv1.LoginRequest) (*authv1.AuthResponse, error) {
		return &authv1.AuthResponse{Tokens: &authv1.TokenPair{
			AccessToken: "access", RefreshToken: "refresh", RefreshExpiresIn: 3600,
		}}, nil
	}}
	w := call(t, NewRouter(&Dependencies{Auth: a, Cookie: cfg}), "POST", "/api/v1/auth/login", `{"email":"a@b.com","password":"ValidPass123"}`, false)
	if w.Code != nethttp.StatusOK || strings.Contains(w.Body.String(), "refresh_token") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if cookie.Name != "fitness_refresh" || cookie.Value != "refresh" || !cookie.HttpOnly || cookie.SameSite != nethttp.SameSiteStrictMode {
		t.Fatalf("cookie=%#v", cookie)
	}
}

func TestRefreshReadsCookieRotatesAndUpdatesCookie(t *testing.T) {
	cfg := RefreshCookieConfig{
		Name:           "fitness_refresh",
		AllowedOrigins: map[string]struct{}{"http://localhost:5173": {}},
	}
	a := authStub{refresh: func(_ context.Context, request *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
		if request.RefreshToken != "old" {
			t.Fatalf("refresh token=%q", request.RefreshToken)
		}
		return &authv1.RefreshResponse{Tokens: &authv1.TokenPair{
			AccessToken: "access-2", RefreshToken: "new", RefreshExpiresIn: 3600,
		}}, nil
	}}
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.AddCookie(&nethttp.Cookie{Name: "fitness_refresh", Value: "old"})
	w := httptest.NewRecorder()
	NewRouter(&Dependencies{Auth: a, Cookie: cfg}).ServeHTTP(w, req)
	if w.Code != nethttp.StatusOK || strings.Contains(w.Body.String(), "refresh_token") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Result().Cookies()[0].Value; got != "new" {
		t.Fatalf("rotated cookie=%q", got)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	cfg := RefreshCookieConfig{
		Name:           "fitness_refresh",
		AllowedOrigins: map[string]struct{}{"http://localhost:5173": {}},
	}
	a := authStub{logout: func(_ context.Context, request *authv1.LogoutRequest) (*authv1.Empty, error) {
		if request.RefreshToken != "refresh" {
			t.Fatalf("refresh token=%q", request.RefreshToken)
		}
		return &authv1.Empty{}, nil
	}}
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Authorization", "Bearer "+token(t, "secret", time.Now().Add(time.Hour)))
	req.AddCookie(&nethttp.Cookie{Name: "fitness_refresh", Value: "refresh"})
	w := httptest.NewRecorder()
	NewRouter(&Dependencies{Auth: a, JWTSecret: "secret", Cookie: cfg}).ServeHTTP(w, req)
	if w.Code != nethttp.StatusNoContent || w.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("status=%d cookies=%#v", w.Code, w.Result().Cookies())
	}
}
