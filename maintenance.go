//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"fmt"
	"math/big"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/rickb777/period"
)

// ITaskSettings3 is a dual interface. Query it explicitly before using its
// IDispatch members; the base ITaskSettings interface need not expose them.
var iidTaskSettings3 = ole.NewGUID("{0AD9D0D7-0C7F-4EBB-9A5F-D1C648DCA528}")

func fillMaintenanceSettingsObj(settings *MaintenanceSettings, settingsObj *ole.IDispatch) error {
	if settings == nil {
		// Ordinary definitions must not require any Windows 8-only properties.
		return nil
	}

	h := &oleHelper{}
	settings3 := h.query(settingsObj, iidTaskSettings3)
	if h.err != nil {
		return fmt.Errorf("error querying ITaskSettings3 for Automatic Maintenance: %w", h.err)
	}
	defer settings3.Release()

	// MS-TSCH requires this for every task with MaintenanceSettings. Leave it
	// untouched for ordinary tasks, which may use features this engine forbids.
	h.put(settings3, "UseUnifiedSchedulingEngine", true)
	if h.err != nil {
		return h.err
	}

	v, err := maintenanceSettingsObject(settings3, true)
	if err != nil {
		return err
	}
	// The VARIANT owns this IDispatch reference. Clear it once; do not also
	// Release the borrowed pointer returned by ToIDispatch.
	defer v.Clear()
	maintenance := v.ToIDispatch()
	h.put(maintenance, "Period", PeriodToString(settings.Period))
	if !settings.Deadline.IsZero() {
		h.put(maintenance, "Deadline", PeriodToString(settings.Deadline))
	}
	h.put(maintenance, "Exclusive", settings.Exclusive)
	return h.err
}

func parseMaintenanceSettings(settingsObj *ole.IDispatch) (*MaintenanceSettings, error) {
	h := &oleHelper{}
	settings3 := h.query(settingsObj, iidTaskSettings3)
	if h.err != nil {
		var oleErr *ole.OleError
		if errors.As(h.err, &oleErr) && oleErr != nil && oleErr.Code() == ole.E_NOINTERFACE {
			// Older schedulers cannot contain maintenance settings. Other COM
			// failures must still reach the caller.
			return nil, nil
		}
		return nil, h.err
	}
	defer settings3.Release()

	v, err := maintenanceSettingsObject(settings3, false)
	if err != nil || v == nil {
		return nil, err
	}
	defer v.Clear()
	maintenance := v.ToIDispatch()
	periodText := h.getString(maintenance, "Period")
	deadlineText := h.getString(maintenance, "Deadline")
	exclusive := h.getBool(maintenance, "Exclusive")
	if h.err != nil {
		return nil, h.err
	}
	maintenancePeriod, err := StringToPeriod(periodText)
	if err != nil {
		return nil, fmt.Errorf("error parsing maintenance Period: %w", err)
	}
	deadline, err := StringToPeriod(deadlineText)
	if err != nil {
		return nil, fmt.Errorf("error parsing maintenance Deadline: %w", err)
	}
	return &MaintenanceSettings{Period: maintenancePeriod, Deadline: deadline, Exclusive: exclusive}, nil
}

// maintenanceSettingsObject returns an owning VARIANT, or nil when no settings
// exist. Callers must Clear a non-nil result. Reading never creates settings.
func maintenanceSettingsObject(settings3 *ole.IDispatch, create bool) (*ole.VARIANT, error) {
	v, err := oleutil.GetProperty(settings3, "MaintenanceSettings")
	if err != nil {
		if v != nil {
			v.Clear()
		}
		return nil, fmt.Errorf("error getting MaintenanceSettings: %w", err)
	}
	absent := v == nil || v.VT == ole.VT_EMPTY || v.VT == ole.VT_NULL || (v.VT == ole.VT_DISPATCH && v.Val == 0)
	if absent {
		if v != nil {
			v.Clear()
		}
		if !create {
			return nil, nil
		}
		v, err = oleutil.CallMethod(settings3, "CreateMaintenanceSettings")
		if err != nil {
			if v != nil {
				v.Clear()
			}
			return nil, fmt.Errorf("error creating MaintenanceSettings: %w", err)
		}
	}
	if v == nil || v.ToIDispatch() == nil {
		if v != nil {
			v.Clear()
		}
		return nil, errors.New("MaintenanceSettings is not an object")
	}
	return v, nil
}

func validateMaintenanceSettings(settings TaskSettings) error {
	m := settings.MaintenanceSettings
	if m == nil {
		return nil
	}
	if settings.Compatibility < TASK_COMPATIBILITY_V2_2 {
		return errors.New("Automatic Maintenance requires TASK_COMPATIBILITY_V2_2 or later")
	}
	months, seconds, err := maintenanceDurationParts(m.Period)
	if err != nil {
		return fmt.Errorf("invalid maintenance Period: %w", err)
	}
	if months.Sign() == 0 && seconds.Cmp(big.NewRat(86400, 1)) < 0 {
		return errors.New("invalid maintenance Period: must be at least one day")
	}
	if m.Deadline.IsZero() {
		return nil
	}
	deadlineMonths, deadlineSeconds, err := maintenanceDurationParts(m.Deadline)
	if err != nil {
		return fmt.Errorf("invalid maintenance Deadline: %w", err)
	}
	if deadlineMonths.Sign() == 0 && deadlineSeconds.Cmp(big.NewRat(86400, 1)) < 0 {
		return errors.New("invalid maintenance Deadline: must be at least one day")
	}
	// Fixed durations compare exactly, without time.Duration overflow. Calendar
	// months/years have no fixed length: reject definite ordering violations,
	// and leave ambiguous cross-unit comparisons to the Windows XML validator.
	if deadlineMonths.Cmp(months) <= 0 && deadlineSeconds.Cmp(seconds) <= 0 {
		return errors.New("invalid maintenance Deadline: must be greater than Period")
	}
	return nil
}

// XML Schema durations contain whole years/months/days/hours/minutes and may
// contain fractional seconds. period.Period also permits weeks and mixed signs,
// which Task Scheduler's duration grammar does not permit.
func maintenanceDurationParts(p period.Period) (months, seconds *big.Rat, err error) {
	months, seconds = new(big.Rat), new(big.Rat)
	for _, field := range []struct {
		kind   period.Designator
		factor int64
	}{
		{period.Year, 12}, {period.Month, 1}, {period.Week, 0},
		{period.Day, 86400}, {period.Hour, 3600}, {period.Minute, 60}, {period.Second, 1},
	} {
		value := p.GetField(field.kind)
		if value.Sign() < 0 {
			return nil, nil, errors.New("duration components must not be negative")
		}
		if field.kind == period.Week && value.Sign() != 0 {
			return nil, nil, errors.New("use days instead of weeks in Task Scheduler durations")
		}
		if field.kind != period.Second && !value.IsInt() {
			return nil, nil, errors.New("only seconds may have a fractional part in Task Scheduler durations")
		}
		part, ok := new(big.Rat).SetString(value.String())
		if !ok {
			return nil, nil, errors.New("invalid duration component")
		}
		part.Mul(part, big.NewRat(field.factor, 1))
		if field.kind == period.Year || field.kind == period.Month {
			months.Add(months, part)
		} else {
			seconds.Add(seconds, part)
		}
	}
	return months, seconds, nil
}
