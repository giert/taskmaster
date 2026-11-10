//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rickb777/period"
)

func TestExecutionTimeLimitsRoundTrip(t *testing.T) {
	taskService := setupTaskService(t)
	tests := []struct {
		name         string
		taskLimit    period.Period
		triggerLimit period.Period
	}{
		{name: "Unlimited"},
		{name: "Finite", taskLimit: period.NewHMS(0, 30, 0), triggerLimit: period.NewHMS(0, 10, 0)},
		{name: "Default72Hours", taskLimit: period.NewHMS(72, 0, 0)},
	}
	// COM objects belong to this goroutine. Keep the registration/read-back loop
	// here rather than sharing the service with t.Run subtest goroutines.
	for _, tt := range tests {
		def := taskService.NewTaskDefinition()
		def.Settings.Enabled = false
		def.Settings.TimeLimit = tt.taskLimit
		def.AddAction(ExecAction{Path: "cmd.exe", Args: "/c exit 0"})
		def.AddTrigger(TimeTrigger{TaskTrigger: TaskTrigger{
			StartBoundary:      time.Now().Add(24 * time.Hour),
			ExecutionTimeLimit: tt.triggerLimit,
			RepetitionPattern: RepetitionPattern{
				RepetitionInterval: period.NewHMS(0, 5, 0),
			},
		}})
		path := testTaskPath("ExecutionLimits", tt.name)
		created, _, err := taskService.CreateTask(path, def, true)
		if err != nil {
			t.Fatalf("%s: create task: %v", tt.name, err)
		}
		created.Release()
		withRegisteredTask(t, taskService, path, func(task RegisteredTask) {
			if got := task.Definition.Settings.TimeLimit; got.DurationApprox() != tt.taskLimit.DurationApprox() {
				t.Errorf("%s: task limit: want %s, got %s", tt.name, tt.taskLimit, got)
			}
			trigger := requireTriggerAt[TimeTrigger](t, task, 0)
			if got := trigger.ExecutionTimeLimit; got.DurationApprox() != tt.triggerLimit.DurationApprox() {
				t.Errorf("%s: trigger limit: want %s, got %s", tt.name, tt.triggerLimit, got)
			}
			// A zero trigger limit must stay unset rather than become an explicit
			// PT0S, so only the task-level element appears in the XML.
			wantElements := 1
			if !tt.triggerLimit.IsZero() {
				wantElements = 2
			}
			if got := strings.Count(task.Definition.XMLText, "<ExecutionTimeLimit>"); got != wantElements {
				t.Errorf("%s: ExecutionTimeLimit elements in XML: want %d, got %d", tt.name, wantElements, got)
			}
			if tt.taskLimit.IsZero() && !strings.Contains(task.Definition.XMLText, "<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>") {
				t.Errorf("%s: unlimited task limit must be written as PT0S", tt.name)
			}
			if got := trigger.RepetitionDuration; !got.IsZero() {
				t.Errorf("%s: indefinite repetition duration: want zero, got %s", tt.name, got)
			}
			if got := trigger.RepetitionInterval; got.DurationApprox() != 5*time.Minute {
				t.Errorf("%s: repetition interval: want PT5M, got %s", tt.name, got)
			}
		})
	}
}

func TestExtendedSchedulesRoundTrip(t *testing.T) {
	taskService := setupTaskService(t)
	start := TaskTrigger{StartBoundary: time.Now().Add(24 * time.Hour)}
	tests := []struct {
		name    string
		trigger Trigger
	}{
		{name: "Every3Days", trigger: DailyTrigger{TaskTrigger: start, DayInterval: 3}},
		{name: "Every256Days", trigger: DailyTrigger{TaskTrigger: start, DayInterval: 256}},
		{name: "Every365Days", trigger: DailyTrigger{TaskTrigger: start, DayInterval: 365}},
		{name: "Every3Weeks", trigger: WeeklyTrigger{TaskTrigger: start, DaysOfWeek: Monday, WeekInterval: 3}},
		{name: "Every52Weeks", trigger: WeeklyTrigger{TaskTrigger: start, DaysOfWeek: Monday, WeekInterval: 52}},
		{name: "LastDayOnly", trigger: MonthlyTrigger{TaskTrigger: start, MonthsOfYear: AllMonths, RunOnLastDayOfMonth: true}},
		{name: "LastDayBit", trigger: MonthlyTrigger{TaskTrigger: start, DaysOfMonth: LastDayOfMonth, MonthsOfYear: AllMonths}},
		{name: "NumberedAndLastDayBit", trigger: MonthlyTrigger{TaskTrigger: start, DaysOfMonth: One | Fifteen | LastDayOfMonth, MonthsOfYear: January | March}},
	}
	for _, tt := range tests {
		def := taskService.NewTaskDefinition()
		def.Settings.Enabled = false
		def.AddAction(ExecAction{Path: "cmd.exe", Args: "/c exit 0"})
		def.AddTrigger(tt.trigger)
		path := testTaskPath("ExtendedSchedules", tt.name)
		created, _, err := taskService.CreateTask(path, def, true)
		if err != nil {
			t.Fatalf("%s: create task: %v", tt.name, err)
		}
		created.Release()
		withRegisteredTask(t, taskService, path, func(task RegisteredTask) {
			switch want := tt.trigger.(type) {
			case DailyTrigger:
				got := requireTriggerAt[DailyTrigger](t, task, 0)
				if got.DayInterval != want.DayInterval {
					t.Errorf("%s: daily interval: want %d, got %d", tt.name, want.DayInterval, got.DayInterval)
				}
			case WeeklyTrigger:
				got := requireTriggerAt[WeeklyTrigger](t, task, 0)
				if got.WeekInterval != want.WeekInterval || got.DaysOfWeek != want.DaysOfWeek {
					t.Errorf("%s: weekly schedule did not round-trip: %+v", tt.name, got)
				}
			case MonthlyTrigger:
				got := requireTriggerAt[MonthlyTrigger](t, task, 0)
				wantLast := want.RunOnLastDayOfMonth || want.DaysOfMonth&LastDayOfMonth != 0
				if got.RunOnLastDayOfMonth != wantLast || got.DaysOfMonth != want.DaysOfMonth&AllDaysOfMonth || got.MonthsOfYear != want.MonthsOfYear {
					t.Errorf("%s: last-day-only schedule did not round-trip: %+v", tt.name, got)
				}
				updated, err := taskService.UpdateTask(path, task.Definition)
				if err != nil {
					t.Fatalf("%s: could not re-register the parsed monthly definition: %v", tt.name, err)
				}
				updated.Release()
			}
		})
	}
}

func TestMissingTaskPreservesNotExist(t *testing.T) {
	taskService := setupTaskService(t)
	_, err := taskService.GetRegisteredTask(testTaskPath("DoesNotExist"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing registered task: want os.ErrNotExist, got %v", err)
	}
	if err := taskService.DeleteTask(testTaskPath("DoesNotExist")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleting missing task: want os.ErrNotExist, got %v", err)
	}
}
