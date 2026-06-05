//go:build windows
// +build windows

package taskmaster

import (
	"testing"
	"time"

	"github.com/rickb777/period"
)

func TestTaskDateToTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantZero  bool
		wantEqual time.Time
		wantErr   bool
	}{
		{name: "empty returns zero", input: "", wantZero: true},
		{name: "no timezone parsed as UTC", input: "2024-01-02T15:04:05", wantEqual: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)},
		{name: "explicit UTC Z", input: "2024-01-02T15:04:05Z", wantEqual: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)},
		{name: "positive offset", input: "2024-01-02T15:04:05+02:00", wantEqual: time.Date(2024, 1, 2, 13, 4, 5, 0, time.UTC)},
		{name: "negative offset", input: "2024-01-02T15:04:05-05:00", wantEqual: time.Date(2024, 1, 2, 20, 4, 5, 0, time.UTC)},
		{name: "invalid", input: "not-a-date", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TaskDateToTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantZero {
				if !got.IsZero() {
					t.Fatalf("expected zero time, got %v", got)
				}
				return
			}
			if !got.Equal(tt.wantEqual) {
				t.Fatalf("expected %v, got %v", tt.wantEqual, got)
			}
		})
	}
}

func TestTimeToTaskDate(t *testing.T) {
	if got := TimeToTaskDate(time.Time{}); got != "" {
		t.Errorf("zero time: want empty string, got %q", got)
	}

	tm := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if got, want := TimeToTaskDate(tm), "2024-03-04T05:06:07"; got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStringToPeriod(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string // expected Period.String()
		wantErr bool
	}{
		{name: "empty is zero period", input: "", want: "P0D"},
		{name: "minutes", input: "PT10M", want: "PT10M"},
		{name: "hours", input: "PT1H", want: "PT1H"},
		{name: "invalid", input: "nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StringToPeriod(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("want %q, got %q", tt.want, got.String())
			}
		})
	}
}

func TestPeriodToString(t *testing.T) {
	if got := PeriodToString(period.Period{}); got != "" {
		t.Errorf("zero period: want empty string, got %q", got)
	}

	if got, want := PeriodToString(period.NewHMS(0, 30, 0)), "PT30M"; got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestIntToDayOfMonth(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		want    DayOfMonth
		wantErr bool
	}{
		{name: "zero invalid", input: 0, wantErr: true},
		{name: "negative invalid", input: -3, wantErr: true},
		{name: "first", input: 1, want: One},
		{name: "second", input: 2, want: Two},
		{name: "thirty-first", input: 31, want: ThirtyOne},
		{name: "thirty-two maps to last day", input: 32, want: LastDayOfMonth},
		{name: "thirty-three invalid", input: 33, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IntToDayOfMonth(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %d, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %d, got %d", tt.want, got)
			}
		})
	}
}
