//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"time"
)

func validateDefinition(def Definition) error {
	var err error

	if def.Actions == nil {
		return ErrNoActions
	}
	if err = validateActions(def.Actions); err != nil {
		return err
	}
	if err = validateTriggers(def.Triggers); err != nil {
		return err
	}

	if def.Principal.UserID != "" && def.Principal.GroupID != "" {
		return ErrInvalidPrincipal
	}

	return nil
}

func validateActions(actions []Action) error {
	for _, action := range actions {
		switch action.GetType() {
		case TASK_ACTION_EXEC, TASK_ACTION_COM_HANDLER:
			// valid; keep validating the remaining actions
		default:
			return errors.New("invalid task action type")
		}
	}

	return nil
}

func validateTriggers(triggers []Trigger) error {
	for _, trigger := range triggers {
		// RepetitionInterval, when set, must be at least one minute; Task Scheduler rejects a smaller value with an opaque "out of range" error.
		if interval := trigger.GetRepetitionInterval(); !interval.IsZero() && interval.DurationApprox() < time.Minute {
			return errors.New("invalid trigger: RepetitionInterval must be at least 1 minute")
		}

		switch t := trigger.(type) {
		case BootTrigger:
			// no required fields
		case DailyTrigger:
			if t.GetStartBoundary().IsZero() {
				return errors.New("invalid DailyTrigger: StartBoundary is required")
			} else if t.DayInterval > EveryOtherDay {
				return errors.New("invalid DailyTrigger: invalid DayInterval")
			}
		case EventTrigger:
			if t.Subscription == "" {
				return errors.New("invalid EventTrigger: Subscription is required")
			}
		case IdleTrigger:
			// no required fields
		case LogonTrigger:
			// no required fields
		case MonthlyDOWTrigger:
			if t.GetStartBoundary().IsZero() {
				return errors.New("invalid MonthlyDOWTrigger: StartBoundary is required")
			} else if t.DaysOfWeek == 0 {
				return errors.New("invalid MonthlyDOWTrigger: DaysOfWeek is required")
			} else if t.DaysOfWeek > AllDays {
				return errors.New("invalid MonthlyDOWTrigger: invalid DaysOfWeek")
			} else if t.MonthsOfYear == 0 {
				return errors.New("invalid MonthlyDOWTrigger: MonthsOfYear is required")
			} else if t.MonthsOfYear > AllMonths {
				return errors.New("invalid MonthlyDOWTrigger: invalid MonthsOfYear")
			} else if t.WeeksOfMonth == 0 {
				return errors.New("invalid MonthlyDOWTrigger: WeeksOfMonth is required")
			} else if t.WeeksOfMonth > AllWeeks {
				return errors.New("invalid MonthlyDOWTrigger: invalid WeeksOfMonth")
			}
		case MonthlyTrigger:
			if t.GetStartBoundary().IsZero() {
				return errors.New("invalid MonthlyTrigger: StartBoundary is required")
			} else if t.DaysOfMonth == 0 {
				return errors.New("invalid MonthlyTrigger: DaysOfMonth is required")
			} else if t.DaysOfMonth > AllDaysOfMonth {
				return errors.New("invalid MonthlyTrigger: invalid DaysOfMonth")
			} else if t.MonthsOfYear == 0 {
				return errors.New("invalid MonthlyTrigger: MonthsOfYear is required")
			} else if t.MonthsOfYear > AllMonths {
				return errors.New("invalid MonthlyTrigger: invalid MonthsOfYear")
			}
		case RegistrationTrigger:
			// no required fields
		case SessionStateChangeTrigger:
			// no required fields
		case TimeTrigger:
			if t.GetStartBoundary().IsZero() {
				return errors.New("invalid TimeTrigger: StartBoundary is required")
			}
		case WeeklyTrigger:
			if t.GetStartBoundary().IsZero() {
				return errors.New("invalid WeeklyTrigger: StartBoundary is required")
			} else if t.DaysOfWeek == 0 {
				return errors.New("invalid WeeklyTrigger: DaysOfWeek is required")
			} else if t.DaysOfWeek > AllDays {
				return errors.New("invalid WeeklyTrigger: invalid DaysOfWeek")
			} else if t.WeekInterval == 0 {
				return errors.New("invalid WeeklyTrigger: WeekInterval is required")
			} else if t.WeekInterval > EveryOtherWeek {
				return errors.New("invalid WeeklyTrigger: invalid WeekInterval")
			}
		default:
			return errors.New("invalid task trigger type")
		}
	}
	return nil
}
