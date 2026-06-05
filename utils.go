//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"strings"
	"time"

	"github.com/rickb777/period"
)

var taskDateFormat = "2006-01-02T15:04:05"
var taskDateFormatWTimeZone = "2006-01-02T15:04:05-07:00"
var taskDateFormatUTC = "2006-01-02T15:04:05Z"

func IntToDayOfMonth(dayOfMonth int) (DayOfMonth, error) {
	if dayOfMonth < 1 || dayOfMonth > 32 {
		return 0, errors.New("invalid day of month")
	}

	return DayOfMonth(1) << (dayOfMonth - 1), nil
}

func TimeToTaskDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format(taskDateFormat)
}

func TaskDateToTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	var t time.Time
	var err error

	if strings.Count(s, "-") == 3 || strings.Contains(s, "+") {
		t, err = time.Parse(taskDateFormatWTimeZone, s)
	} else if s[len(s)-1] == 'Z' {
		t, err = time.Parse(taskDateFormatUTC, s)
	} else {
		t, err = time.Parse(taskDateFormat, s)
	}
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

func StringToPeriod(s string) (period.Period, error) {
	if s == "" {
		return period.Period{}, nil
	}

	return period.Parse(s)
}

func PeriodToString(p period.Period) string {
	s := p.String()
	if s == "P0D" {
		return ""
	}

	return s
}
