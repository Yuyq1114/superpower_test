package workoutevent

import (
	"errors"
	"fmt"
)

const TypeWorkoutCompletedV1 = "workout.completed.v1"

type Event struct {
	EventID, EventType, UserID, CheckinID, CompletedAt, OccurredAt string
}

func Fields(e Event) map[string]any {
	return map[string]any{"event_id": e.EventID, "event_type": e.EventType, "user_id": e.UserID, "checkin_id": e.CheckinID, "completed_at": e.CompletedAt, "occurred_at": e.OccurredAt}
}

func Parse(values map[string]any) (Event, error) {
	value := func(key string) string {
		if v, ok := values[key]; ok {
			return fmt.Sprint(v)
		}
		return ""
	}
	e := Event{EventID: value("event_id"), EventType: value("event_type"), UserID: value("user_id"), CheckinID: value("checkin_id"), CompletedAt: value("completed_at"), OccurredAt: value("occurred_at")}
	if e.EventID == "" || e.EventType == "" || e.UserID == "" || e.CheckinID == "" || e.CompletedAt == "" || e.OccurredAt == "" {
		return Event{}, errors.New("missing stable event fields")
	}
	return e, nil
}
