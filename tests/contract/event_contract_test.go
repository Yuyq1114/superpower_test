package contract

import (
	"encoding/json"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestWorkoutCompletedEventContractRoundTrip(t *testing.T) {
	original := &checkinv1.WorkoutCompleted{EventId: "event-1", EventType: "workout.completed.v1", UserId: "user-1", CheckinId: "checkin-1", CompletedAt: "2026-08-08T03:00:00Z", OccurredAt: "2026-08-08T03:00:01Z"}
	payload, err := proto.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded checkinv1.WorkoutCompleted
	if err := proto.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(original, &decoded) {
		t.Fatalf("event changed after protobuf round trip: got=%v want=%v", &decoded, original)
	}
	if decoded.EventId == "" || decoded.CheckinId == "" {
		t.Fatal("event identity fields must remain available for consumer deduplication")
	}
}
func TestWorkoutCompletedJSONUsesStableSnakeCaseFields(t *testing.T) {
	payload, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(&checkinv1.WorkoutCompleted{EventId: "event-1", EventType: "workout.completed.v1", UserId: "user-1", CheckinId: "checkin-1"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"event_id": "event-1", "event_type": "workout.completed.v1", "user_id": "user-1", "checkin_id": "checkin-1"}
	if len(got) != len(want) {
		t.Fatalf("event JSON contract changed: got=%v want=%v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("event JSON contract changed: got=%v want=%v", got, want)
		}
	}
}
