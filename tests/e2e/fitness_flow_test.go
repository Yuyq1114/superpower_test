package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type client struct {
	baseURL, token string
	http           *http.Client
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
		t.Fatal("BASE_URL is required for E2E tests; start the complete stack and run `BASE_URL=http://127.0.0.1:8080 make test-e2e`")
	}
	c := &client{baseURL: base, http: &http.Client{Timeout: 5 * time.Second}}
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
	must(t, c.do(ctx, "POST", "/api/v1/auth/login", map[string]any{"email": email, "password": password}, "", 200, &logged))
	c.token = logged.Tokens.AccessToken
	if c.token == "" {
		t.Fatal("empty login access token")
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
	resp, e := c.http.Do(req)
	if e != nil {
		return fmt.Errorf("call %s %s: %w", method, path, e)
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(resp.Body)
	if e != nil {
		return e
	}
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
