package billing

import (
	"testing"
	"time"
)

func TestServerLimitFollowsPaidPeriod(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, tc := range []struct {
		name   string
		record Record
		want   int
	}{
		{"free", Record{}, 1},
		{"paid", Record{Status: "active", PeriodEnd: now.Unix() + 1}, 10},
		{"cancel_at_end", Record{Status: "active", PeriodEnd: now.Unix() + 1, CancelAtPeriodEnd: true}, 10},
		{"expired", Record{Status: "active", PeriodEnd: now.Unix()}, 1},
		{"past_due", Record{Status: "past_due", PeriodEnd: now.Unix() + 1}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.record.ServerLimit(now); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}
