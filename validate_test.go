//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"testing"
	"time"
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
		{name: "monthly ok", triggers: []Trigger{MonthlyTrigger{DaysOfMonth: One, MonthsOfYear: January, TaskTrigger: withStart}}},
		{name: "monthly dow ok", triggers: []Trigger{MonthlyDOWTrigger{DaysOfWeek: Monday, WeeksOfMonth: First, MonthsOfYear: January, TaskTrigger: withStart}}},
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
