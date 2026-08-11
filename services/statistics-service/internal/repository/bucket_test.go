package repository

import (
	"testing"
	"time"

	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
)

// The whole point of publishing the logical workout date as the event's
// `completed_at` (see services/checkin-service/internal/service.Complete) is
// that this bucketing then lands a backfilled check-in in the week it was
// actually trained, not the week it was typed in.
func TestWeekBucketFollowsTheLogicalWorkoutDateNotTheWriteInstant(t *testing.T) {
	logicalDate := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)    // Wednesday
	writeInstant := time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC) // the following Tuesday

	trainedWeek := bucketStart(model.PeriodWeek, logicalDate)
	writtenWeek := bucketStart(model.PeriodWeek, writeInstant)

	if want := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC); !trainedWeek.Equal(want) {
		t.Fatalf("bucketStart(%s) = %s, want the ISO week starting %s", logicalDate, trainedWeek, want)
	}
	if trainedWeek.Equal(writtenWeek) {
		t.Fatal("this fixture is meaningless unless the workout date and the write instant fall in different weeks")
	}
}

func TestWeekAndDayBucketsAreISOMondayAligned(t *testing.T) {
	for _, tt := range []struct {
		name       string
		at         time.Time
		wantWeek   time.Time
		wantDayUTC time.Time
	}{
		{
			name:       "monday is its own week start",
			at:         time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
			wantWeek:   time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
			wantDayUTC: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name:       "sunday belongs to the week that started six days earlier",
			at:         time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
			wantWeek:   time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
			wantDayUTC: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:       "an ISO week spanning new year keeps its december monday",
			at:         time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantWeek:   time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC),
			wantDayUTC: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := bucketStart(model.PeriodWeek, tt.at); !got.Equal(tt.wantWeek) {
				t.Errorf("bucketStart(week, %s) = %s, want %s", tt.at, got, tt.wantWeek)
			}
			if got := dayStart(tt.at); !got.Equal(tt.wantDayUTC) {
				t.Errorf("dayStart(%s) = %s, want %s", tt.at, got, tt.wantDayUTC)
			}
		})
	}
}
