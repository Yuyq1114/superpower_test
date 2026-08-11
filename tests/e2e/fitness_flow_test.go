//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	urlpkg "net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type client struct {
	baseURL, token string
	http           *http.Client
	lastBody       []byte
}

func newClient(base string) (*client, error) {
	jar, e := cookiejar.New(nil)
	if e != nil {
		return nil, e
	}
	return &client{baseURL: base, http: &http.Client{Timeout: 5 * time.Second, Jar: jar}}, nil
}
type authResponse struct {
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}
type planResponse struct {
	Plan struct {
		ID string `json:"id"`
	} `json:"plan"`
}
type dayResponse struct {
	WorkoutDay struct {
		ID string `json:"id"`
	} `json:"workout_day"`
}
type itemResponse struct {
	Item struct {
		ID string `json:"id"`
	} `json:"item"`
}
type checkinResponse struct {
	Checkin struct {
		ID string `json:"id"`
	} `json:"checkin"`
}
type historyResponse struct {
	Checkins []struct {
		ID string `json:"id"`
	} `json:"checkins"`
}
type metricResponse struct {
	Metric struct {
		ID string `json:"id"`
	} `json:"metric"`
}
type summaryResponse struct {
	Summary struct {
		WorkoutCount, ActiveDays int64 `json:"-"`
	} `json:"summary"`
}

func TestFitnessFlow(t *testing.T) {
	base := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if base == "" {
		t.Fatal("BASE_URL is required for E2E tests; since Task 8, api-gateway no longer publishes a host port, so start the complete stack and run `BASE_URL=http://127.0.0.1:8088 make test-e2e` against the same-origin frontend entrypoint")
	}
	c, err := newClient(base)
	if err != nil {
		t.Fatalf("build HTTP client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := c.do(ctx, "GET", "/healthz", nil, "", 200, nil); err != nil {
		t.Fatalf("Gateway at BASE_URL=%q is unavailable: %v", base, err)
	}
	suffix := uuid.NewString()
	email, password := "e2e-"+suffix+"@example.test", "E2e-password-123!"
	var registered, logged authResponse
	must(t, c.do(ctx, "POST", "/api/v1/auth/register", map[string]any{"email": email, "password": password}, "", 201, &registered))
	if registered.User.ID == "" {
		t.Fatal("empty registered user id")
	}
	if strings.Contains(string(c.lastBody), "refresh_token") {
		t.Fatalf("register response leaked refresh_token: %s", c.lastBody)
	}
	must(t, c.do(ctx, "POST", "/api/v1/auth/login", map[string]any{"email": email, "password": password}, "", 200, &logged))
	c.token = logged.Tokens.AccessToken
	if c.token == "" {
		t.Fatal("empty login access token")
	}
	if strings.Contains(string(c.lastBody), "refresh_token") {
		t.Fatalf("login response leaked refresh_token: %s", c.lastBody)
	}
	var plan planResponse
	must(t, c.do(ctx, "POST", "/api/v1/plans", map[string]any{"name": "E2E strength plan"}, "plan-"+suffix, 201, &plan))
	if plan.Plan.ID == "" {
		t.Fatal("empty plan id")
	}
	today := time.Now().UTC().Format("2006-01-02")
	var day dayResponse
	must(t, c.do(ctx, "POST", "/api/v1/plans/"+plan.Plan.ID+"/days", map[string]any{"date": today}, "day-"+suffix, 201, &day))
	if day.WorkoutDay.ID == "" {
		t.Fatal("empty day id")
	}
	var item itemResponse
	must(t, c.do(ctx, "POST", "/api/v1/workout-days/"+day.WorkoutDay.ID+"/items", map[string]any{"item": map[string]any{"name": "Squat", "sets": 3, "repetitions": 5, "weight": 80, "duration_seconds": 600}}, "item-"+suffix, 201, &item))
	if item.Item.ID == "" {
		t.Fatal("empty item id")
	}
	var fetchedPlan, fetchedDay, fetchedItem map[string]any
	must(t, c.do(ctx, "GET", "/api/v1/plans/"+plan.Plan.ID, nil, "", 200, &fetchedPlan))
	if fetchedPlan["plan"].(map[string]any)["id"] != plan.Plan.ID || fetchedPlan["plan"].(map[string]any)["name"] != "E2E strength plan" {
		t.Fatalf("plan query lost identity or name: %#v", fetchedPlan)
	}
	must(t, c.do(ctx, "GET", "/api/v1/plans/"+plan.Plan.ID+"/days/"+day.WorkoutDay.ID, nil, "", 200, &fetchedDay))
	if fetchedDay["workout_day"].(map[string]any)["id"] != day.WorkoutDay.ID || fetchedDay["workout_day"].(map[string]any)["plan_id"] != plan.Plan.ID {
		t.Fatalf("day query lost parent: %#v", fetchedDay)
	}
	must(t, c.do(ctx, "GET", "/api/v1/workout-days/"+day.WorkoutDay.ID+"/items/"+item.Item.ID, nil, "", 200, &fetchedItem))
	if fetchedItem["item"].(map[string]any)["id"] != item.Item.ID || fetchedItem["item"].(map[string]any)["workout_day_id"] != day.WorkoutDay.ID || fetchedItem["item"].(map[string]any)["name"] != "Squat" {
		t.Fatalf("item query lost parent or name: %#v", fetchedItem)
	}
	body := map[string]any{"workout_item_id": item.Item.ID, "date": today, "note": "e2e"}
	key := "checkin-" + suffix
	var first, duplicate checkinResponse
	must(t, c.do(ctx, "POST", "/api/v1/checkins", body, key, 201, &first))
	must(t, c.do(ctx, "POST", "/api/v1/checkins", body, key, 201, &duplicate))
	if first.Checkin.ID == "" || first.Checkin.ID != duplicate.Checkin.ID {
		t.Fatalf("idempotent check-in IDs differ: %q and %q", first.Checkin.ID, duplicate.Checkin.ID)
	}
	var history historyResponse
	must(t, c.do(ctx, "GET", "/api/v1/checkins?from="+today+"&to="+today, nil, "", 200, &history))
	if len(history.Checkins) != 1 || history.Checkins[0].ID != first.Checkin.ID {
		t.Fatalf("duplicate check-in was counted more than once: %#v", history.Checkins)
	}
	var metric metricResponse
	must(t, c.do(ctx, "POST", "/api/v1/body-metrics", map[string]any{"metric_type": "weight", "value": 70.5, "unit": "kg", "recorded_at": time.Now().UTC().Format(time.RFC3339)}, "metric-"+suffix, 201, &metric))
	if metric.Metric.ID == "" {
		t.Fatal("empty metric id")
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		var summary struct {
			Summary struct {
				WorkoutCount int64 `json:"workout_count"`
				ActiveDays   int64 `json:"active_days"`
			} `json:"summary"`
		}
		err := c.do(ctx, "GET", "/api/v1/statistics/summary?period=week&start="+time.Now().UTC().Format(time.RFC3339), nil, "", 200, &summary)
		if err == nil && summary.Summary.WorkoutCount == 1 && summary.Summary.ActiveDays == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("statistics did not converge to one event: summary=%+v err=%v", summary.Summary, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
func TestRefreshCookieRotationAndLogout(t *testing.T) {
	base := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if base == "" {
		t.Fatal("BASE_URL is required for E2E tests; since Task 8, api-gateway no longer publishes a host port, so start the complete stack and run `BASE_URL=http://127.0.0.1:8088 make test-e2e` against the same-origin frontend entrypoint")
	}
	c, err := newClient(base)
	if err != nil {
		t.Fatalf("build HTTP client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := uuid.NewString()
	email, password := "e2e-refresh-"+suffix+"@example.test", "E2e-password-123!"
	var logged authResponse
	must(t, c.do(ctx, "POST", "/api/v1/auth/register", map[string]any{"email": email, "password": password}, "", 201, nil))
	must(t, c.do(ctx, "POST", "/api/v1/auth/login", map[string]any{"email": email, "password": password}, "", 200, &logged))
	c.token = logged.Tokens.AccessToken
	if c.token == "" {
		t.Fatal("empty login access token")
	}
	loginCookies := c.http.Jar.Cookies(mustURL(t, base))
	if !hasCookie(loginCookies, "fitness_refresh") {
		t.Fatal("login response did not set fitness_refresh cookie")
	}

	var refreshed authResponse
	must(t, c.do(ctx, "POST", "/api/v1/auth/refresh", nil, "", 200, &refreshed))
	if strings.Contains(string(c.lastBody), "refresh_token") {
		t.Fatalf("refresh response leaked refresh_token: %s", c.lastBody)
	}
	if refreshed.Tokens.AccessToken == "" || refreshed.Tokens.AccessToken == c.token {
		t.Fatalf("refresh did not rotate access token: %s", c.lastBody)
	}
	c.token = refreshed.Tokens.AccessToken

	rotatedCookies := c.http.Jar.Cookies(mustURL(t, base))
	if !hasCookie(rotatedCookies, "fitness_refresh") {
		t.Fatal("refresh response did not keep a fitness_refresh cookie")
	}

	if e := c.do(ctx, "POST", "/api/v1/auth/logout", nil, "", 204, nil); e != nil {
		t.Fatalf("logout failed: %v", e)
	}
	postLogoutCookies := c.http.Jar.Cookies(mustURL(t, base))
	if hasCookie(postLogoutCookies, "fitness_refresh") {
		t.Fatal("logout did not clear the fitness_refresh cookie")
	}

	if e := c.do(ctx, "POST", "/api/v1/auth/refresh", nil, "", 200, nil); e == nil {
		t.Fatal("refresh succeeded after logout, expected the rotated cookie to be invalidated")
	}
}
func hasCookie(cookies []*http.Cookie, name string) bool {
	for _, ck := range cookies {
		if ck.Name == name && ck.Value != "" {
			return true
		}
	}
	return false
}
func mustURL(t *testing.T, raw string) *urlpkg.URL {
	t.Helper()
	u, e := urlpkg.Parse(raw)
	if e != nil {
		t.Fatalf("parse BASE_URL: %v", e)
	}
	return u
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func (c *client) do(ctx context.Context, method, path string, body any, key string, want int, out any) error {
	var r io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		r = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if e != nil {
		return e
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req.Header.Set("Origin", c.baseURL)
	resp, e := c.http.Do(req)
	if e != nil {
		return fmt.Errorf("call %s %s: %w", method, path, e)
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(resp.Body)
	if e != nil {
		return e
	}
	c.lastBody = data
	if resp.StatusCode != want {
		return fmt.Errorf("%s %s returned %d, want %d: %s", method, path, resp.StatusCode, want, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		if e := json.Unmarshal(data, out); e != nil {
			return fmt.Errorf("decode %s: %w", path, e)
		}
	}
	return nil
}
