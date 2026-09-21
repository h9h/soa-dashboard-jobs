package httpapi

import (
	"testing"
	"time"
)

func TestRelativeTimeMatchesMomentBuckets(t *testing.T) {
	cases := []struct {
		duration time.Duration
		want     string
	}{
		{0, "a few seconds ago"},
		{44 * time.Second, "a few seconds ago"},
		{45 * time.Second, "a minute ago"},
		{89 * time.Second, "a minute ago"},
		{90 * time.Second, "2 minutes ago"},
		{44 * time.Minute, "44 minutes ago"},
		{45 * time.Minute, "an hour ago"},
		{89 * time.Minute, "an hour ago"},
		{90 * time.Minute, "2 hours ago"},
		{21 * time.Hour, "21 hours ago"},
		{22 * time.Hour, "a day ago"},
		{35 * time.Hour, "a day ago"},
		{36 * time.Hour, "2 days ago"},
		{25 * 24 * time.Hour, "25 days ago"},
		{26 * 24 * time.Hour, "a month ago"},
		{45 * 24 * time.Hour, "a month ago"},
		{60 * 24 * time.Hour, "2 months ago"},
		{75 * 24 * time.Hour, "2 months ago"},
		{289 * 24 * time.Hour, "9 months ago"},
		{315 * 24 * time.Hour, "10 months ago"},
		{319 * 24 * time.Hour, "10 months ago"},
		{320 * 24 * time.Hour, "a year ago"},
		{547 * 24 * time.Hour, "a year ago"},
		{548 * 24 * time.Hour, "2 years ago"},
	}

	for _, tc := range cases {
		if got := relativeTime(tc.duration); got != tc.want {
			t.Errorf("relativeTime(%v) = %q, erwartet %q", tc.duration, got, tc.want)
		}
	}
}

func TestRelativeTimeHandlesNegativeDurations(t *testing.T) {
	if got := relativeTime(-5 * time.Second); got != "a few seconds ago" {
		t.Errorf("relativeTime(negativ) = %q", got)
	}
}

func TestRelativeTimeFractionalDurations(t *testing.T) {
	cases := []struct {
		duration time.Duration
		want     string
	}{
		// Exact regression from re-review: 289.3 days must be "10 months ago", not "9 months ago"
		{6943*time.Hour + 12*time.Minute, "10 months ago"},
		// Inside the window where day-based arm and moment disagreed: 319.55 days
		{7669*time.Hour + 12*time.Minute, "10 months ago"},
		// Fractional seconds: must be "a minute ago", not "1 minutes ago"
		{89*time.Second + 750*time.Millisecond, "a minute ago"},
		// Fractional minutes: must be "an hour ago", not "1 hours ago"
		{89*time.Minute + 45*time.Second, "an hour ago"},
		// Fractional hours: must be "a day ago", not "1 days ago"
		{35*time.Hour + 45*time.Minute, "a day ago"},
	}

	for _, tc := range cases {
		if got := relativeTime(tc.duration); got != tc.want {
			t.Errorf("relativeTime(%v) = %q, erwartet %q", tc.duration, got, tc.want)
		}
	}
}
