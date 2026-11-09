//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"testing"
	"time"

	"github.com/rickb777/period"
)

// fakeAction is an Action whose type is not supported, used to exercise the
// validation error path without a live Task Scheduler.
type fakeAction struct{}

func (fakeAction) GetID() string           { return "" }
func (fakeAction) GetType() TaskActionType { return TaskActionType(999) }

// fakeTrigger is a Trigger whose type is not supported. It embeds TaskTrigger to
// satisfy the rest of the Trigger interface.
type fakeTrigger struct {
	TaskTrigger
}

func (fakeTrigger) GetType() TaskTriggerType { return TaskTriggerType(999) }

func TestValidateActions(t *testing.T) {
	tests := []struct {
		name    string
		actions []Action
		wantErr bool
	}{
		{name: "exec", actions: []Action{ExecAction{Path: "cmd.exe"}}},
		{name: "com handler", actions: []Action{ComHandlerAction{ClassID: "{F0001111-0000-0000-0000-0000FEEDACDC}"}}},
		{name: "multiple valid", actions: []Action{ExecAction{}, ComHandlerAction{}}},
		{name: "unsupported type", actions: []Action{fakeAction{}}, wantErr: true},
		// regression: a valid first action must not mask an invalid later one
		{name: "valid then invalid", actions: []Action{ExecAction{}, fakeAction{}}, wantErr: true},
		{name: "empty", actions: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateActions(tt.actions)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateTriggers(t *testing.T) {
	withStart := TaskTrigger{StartBoundary: time.Now()}

	tests := []struct {
		name     string
		triggers []Trigger
		wantErr  bool
	}{
		{name: "boot", triggers: []Trigger{BootTrigger{}}},
		{name: "daily ok", triggers: []Trigger{DailyTrigger{DayInterval: EveryDay, TaskTrigger: withStart}}},
		{name: "daily missing start boundary", triggers: []Trigger{DailyTrigger{DayInterval: EveryDay}}, wantErr: true},
		{name: "event ok", triggers: []Trigger{EventTrigger{Subscription: "<QueryList/>"}}},
		{name: "event missing subscription", triggers: []Trigger{EventTrigger{}}, wantErr: true},
		{name: "weekly ok", triggers: []Trigger{WeeklyTrigger{DaysOfWeek: Monday, WeekInterval: EveryWeek, TaskTrigger: withStart}}},
		{name: "weekly missing days", triggers: []Trigger{WeeklyTrigger{WeekInterval: EveryWeek, TaskTrigger: withStart}}, wantErr: true},
		{name: "weekly combined days", triggers: []Trigger{WeeklyTrigger{DaysOfWeek: Monday | Friday, WeekInterval: EveryWeek, TaskTrigger: withStart}}},
		{name: "weekly invalid day mask", triggers: []Trigger{WeeklyTrigger{DaysOfWeek: AllDays + 1, WeekInterval: EveryWeek, TaskTrigger: withStart}}, wantErr: true},
		{name: "monthly ok", triggers: []Trigger{MonthlyTrigger{DaysOfMonth: One, MonthsOfYear: January, TaskTrigger: withStart}}},
		{name: "monthly dow ok", triggers: []Trigger{MonthlyDOWTrigger{DaysOfWeek: Monday, WeeksOfMonth: First, MonthsOfYear: January, TaskTrigger: withStart}}},
		{name: "monthly dow invalid day mask", triggers: []Trigger{MonthlyDOWTrigger{DaysOfWeek: AllDays + 1, WeeksOfMonth: First, MonthsOfYear: January, TaskTrigger: withStart}}, wantErr: true},
		{name: "monthly dow invalid week mask", triggers: []Trigger{MonthlyDOWTrigger{DaysOfWeek: Monday, WeeksOfMonth: AllWeeks + 1, MonthsOfYear: January, TaskTrigger: withStart}}, wantErr: true},
		{name: "monthly dow invalid month mask", triggers: []Trigger{MonthlyDOWTrigger{DaysOfWeek: Monday, WeeksOfMonth: First, MonthsOfYear: AllMonths + 1, TaskTrigger: withStart}}, wantErr: true},
		{name: "time ok", triggers: []Trigger{TimeTrigger{TaskTrigger: withStart}}},
		{name: "time missing start boundary", triggers: []Trigger{TimeTrigger{}}, wantErr: true},
		{name: "sub-minute repetition interval", triggers: []Trigger{BootTrigger{TaskTrigger: TaskTrigger{RepetitionPattern: RepetitionPattern{RepetitionInterval: period.NewHMS(0, 0, 30)}}}}, wantErr: true},
		{name: "one-minute repetition interval ok", triggers: []Trigger{BootTrigger{TaskTrigger: TaskTrigger{RepetitionPattern: RepetitionPattern{RepetitionInterval: period.NewHMS(0, 1, 0)}}}}},
		{name: "unsupported type", triggers: []Trigger{fakeTrigger{}}, wantErr: true},
		// regression: a valid first trigger must not mask an invalid later one
		{name: "valid then invalid", triggers: []Trigger{BootTrigger{}, DailyTrigger{DayInterval: EveryDay}}, wantErr: true},
		{name: "empty", triggers: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTriggers(tt.triggers)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateDailyIntervals(t *testing.T) {
	withStart := TaskTrigger{StartBoundary: time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)}
	tests := []struct {
		name     string
		interval DayInterval
		wantErr  bool
	}{
		{name: "zero days", interval: 0, wantErr: true},
		{name: "one day", interval: 1},
		{name: "two days", interval: 2},
		{name: "three days", interval: 3},
		{name: "255 days", interval: 255},
		{name: "256 days", interval: 256},
		{name: "365 days", interval: 365},
		{name: "366 days", interval: 366, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := DailyTrigger{TaskTrigger: withStart, DayInterval: tt.interval}
			err := validateTriggers([]Trigger{trigger})
			if (err != nil) != tt.wantErr {
				t.Fatalf("DayInterval=%d: wantErr=%v, got err=%v", tt.interval, tt.wantErr, err)
			}
		})
	}
}

func TestValidateWeeklyIntervals(t *testing.T) {
	withStart := TaskTrigger{StartBoundary: time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)}
	tests := []struct {
		name     string
		interval WeekInterval
		wantErr  bool
	}{
		{name: "zero weeks", interval: 0, wantErr: true},
		{name: "one week", interval: 1},
		{name: "two weeks", interval: 2},
		{name: "three weeks", interval: 3},
		{name: "52 weeks", interval: 52},
		{name: "53 weeks", interval: 53, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := WeeklyTrigger{TaskTrigger: withStart, DaysOfWeek: Monday, WeekInterval: tt.interval}
			err := validateTriggers([]Trigger{trigger})
			if (err != nil) != tt.wantErr {
				t.Fatalf("WeekInterval=%d: wantErr=%v, got err=%v", tt.interval, tt.wantErr, err)
			}
		})
	}
}

func TestValidateMonthlyLastDay(t *testing.T) {
	withStart := TaskTrigger{StartBoundary: time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)}
	tests := []struct {
		name                string
		days                DayOfMonth
		months              Month
		runOnLastDayOfMonth bool
		wantErr             bool
	}{
		{name: "numbered day only", days: One, months: January},
		{name: "last day only", months: January, runOnLastDayOfMonth: true},
		{name: "last day in all months", months: AllMonths, runOnLastDayOfMonth: true},
		{name: "numbered days and last day", days: One | Fifteen, months: January | March, runOnLastDayOfMonth: true},
		{name: "last day mask only", days: LastDayOfMonth, months: January},
		{name: "numbered day and last day mask", days: One | LastDayOfMonth, months: January},
		{name: "no days selected", months: January, wantErr: true},
		{name: "last day without months", runOnLastDayOfMonth: true, wantErr: true},
		{name: "last day with invalid month mask", months: AllMonths + 1, runOnLastDayOfMonth: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := MonthlyTrigger{
				TaskTrigger:         withStart,
				DaysOfMonth:         tt.days,
				MonthsOfYear:        tt.months,
				RunOnLastDayOfMonth: tt.runOnLastDayOfMonth,
			}
			err := validateTriggers([]Trigger{trigger})
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateDefinition(t *testing.T) {
	t.Run("no actions returns ErrNoActions", func(t *testing.T) {
		if err := validateDefinition(Definition{}); !errors.Is(err, ErrNoActions) {
			t.Fatalf("want ErrNoActions, got %v", err)
		}
	})

	t.Run("UserID and GroupID are mutually exclusive", func(t *testing.T) {
		def := Definition{
			Actions:   []Action{ExecAction{Path: "cmd.exe"}},
			Principal: Principal{UserID: "user", GroupID: "group"},
		}
		if err := validateDefinition(def); !errors.Is(err, ErrInvalidPrincipal) {
			t.Fatalf("want ErrInvalidPrincipal, got %v", err)
		}
	})

	t.Run("valid definition", func(t *testing.T) {
		def := Definition{Actions: []Action{ExecAction{Path: "cmd.exe"}}}
		if err := validateDefinition(def); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
