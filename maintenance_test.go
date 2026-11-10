//go:build windows
// +build windows

package taskmaster

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/rickb777/period"
)

func TestMaintenanceSettingsValidation(t *testing.T) {
	day := period.NewYMD(0, 0, 1)
	fractionalDay, err := period.Parse("P1.5D")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name          string
		settings      *MaintenanceSettings
		compatibility TaskCompatibility
		wantError     string
	}{
		{name: "ordinary task keeps old compatibility", compatibility: TASK_COMPATIBILITY_V2},
		{name: "period only", settings: &MaintenanceSettings{Period: day}},
		{name: "deadline", settings: &MaintenanceSettings{Period: day, Deadline: period.NewYMD(0, 0, 7)}},
		{name: "exclusive", settings: &MaintenanceSettings{Period: day, Exclusive: true}},
		{name: "too old compatibility", settings: &MaintenanceSettings{Period: day}, compatibility: TASK_COMPATIBILITY_V2, wantError: "TASK_COMPATIBILITY_V2_2"},
		{name: "missing period", settings: &MaintenanceSettings{}, wantError: "Period"},
		{name: "negative period", settings: &MaintenanceSettings{Period: day.Negate()}, wantError: "Period"},
		{name: "mixed signs", settings: &MaintenanceSettings{Period: period.New(0, 0, 0, 2, -1, 0, 0)}, wantError: "negative"},
		{name: "below one day", settings: &MaintenanceSettings{Period: period.NewHMS(23, 59, 59)}, wantError: "one day"},
		{name: "24 hours", settings: &MaintenanceSettings{Period: period.NewHMS(24, 0, 0)}},
		{name: "fractional seconds", settings: &MaintenanceSettings{Period: period.NewOf(24*time.Hour + time.Second/2)}},
		{name: "fractional days are not XML durations", settings: &MaintenanceSettings{Period: fractionalDay}, wantError: "fractional"},
		{name: "equal deadline", settings: &MaintenanceSettings{Period: day, Deadline: day}, wantError: "greater"},
		{name: "equivalent deadline", settings: &MaintenanceSettings{Period: day, Deadline: period.NewHMS(24, 0, 0)}, wantError: "greater"},
		{name: "earlier deadline", settings: &MaintenanceSettings{Period: period.NewYMD(0, 0, 2), Deadline: day}, wantError: "greater"},
		{name: "negative deadline", settings: &MaintenanceSettings{Period: day, Deadline: day.Negate()}, wantError: "Deadline"},
		{name: "deadline below a day", settings: &MaintenanceSettings{Period: period.NewYMD(0, 1, 0), Deadline: period.NewHMS(23, 0, 0)}, wantError: "one day"},
		{name: "deadline one second later", settings: &MaintenanceSettings{Period: day, Deadline: period.New(0, 0, 0, 1, 0, 0, 1)}},
		{name: "weeks are not XML durations", settings: &MaintenanceSettings{Period: period.NewYMWD(0, 0, 1, 0)}, wantError: "days instead of weeks"},
		{name: "calendar period", settings: &MaintenanceSettings{Period: period.NewYMD(0, 1, 0)}},
		{name: "equivalent calendar deadline", settings: &MaintenanceSettings{Period: period.NewYMD(1, 0, 0), Deadline: period.NewYMD(0, 12, 0)}, wantError: "greater"},
		{name: "calendar ordering left to Windows", settings: &MaintenanceSettings{Period: period.NewYMD(0, 1, 0), Deadline: period.NewYMD(0, 0, 31)}},
		{name: "large durations do not overflow", settings: &MaintenanceSettings{Period: period.NewYMD(0, 0, 1000000), Deadline: period.NewYMD(0, 0, 1000001)}},
		{name: "large equal deadline", settings: &MaintenanceSettings{Period: period.NewYMD(0, 0, 1000000), Deadline: period.NewYMD(0, 0, 1000000)}, wantError: "greater"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			def := DefaultDefinition()
			def.AddAction(ExecAction{Path: "cmd.exe"})
			def.Settings.MaintenanceSettings = tt.settings
			def.Settings.Compatibility = tt.compatibility
			if tt.compatibility == 0 {
				def.Settings.Compatibility = TASK_COMPATIBILITY_V2_2
			}
			err := validateDefinition(def)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected validation error: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("want an error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestMaintenanceSettingsNilWrite(t *testing.T) {
	// A nil object makes any accidental COM access fail immediately. Omitting
	// maintenance must be safe without an ITaskSettings3 interface at all.
	if err := fillMaintenanceSettingsObj(nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMaintenanceSettingsUnsupportedInterface(t *testing.T) {
	service, err := Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer service.Disconnect()

	// The service is a real COM object that does not implement ITaskSettings3.
	// Reading an unsupported interface is optional; explicitly requesting the
	// feature must preserve the native error for errors.As callers.
	settings, err := parseMaintenanceSettings(service.taskServiceObj)
	if err != nil || settings != nil {
		t.Fatalf("unsupported read: want nil settings, nil error; got %v, %v", settings, err)
	}
	err = fillMaintenanceSettingsObj(&MaintenanceSettings{Period: period.NewYMD(0, 0, 1)}, service.taskServiceObj)
	var oleErr *ole.OleError
	if !errors.As(err, &oleErr) || oleErr.Code() != ole.E_NOINTERFACE {
		t.Fatalf("unsupported write: want wrapped E_NOINTERFACE, got %v", err)
	}
}

func TestMaintenanceSettingsRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name          string
		settings      *MaintenanceSettings
		compatibility TaskCompatibility
	}{
		{name: "ordinary V2 task", compatibility: TASK_COMPATIBILITY_V2},
		{name: "ordinary V2_2 task", compatibility: TASK_COMPATIBILITY_V2_2},
		{name: "period only", compatibility: TASK_COMPATIBILITY_V2_2, settings: &MaintenanceSettings{Period: period.NewYMD(0, 0, 1)}},
		{name: "with deadline", compatibility: TASK_COMPATIBILITY_V2_2, settings: &MaintenanceSettings{Period: period.NewYMD(0, 0, 2), Deadline: period.NewYMD(0, 0, 14)}},
		{name: "exclusive", compatibility: TASK_COMPATIBILITY_V2_2, settings: &MaintenanceSettings{Period: period.NewYMD(0, 0, 5), Deadline: period.NewYMD(0, 0, 15), Exclusive: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Connect inside the subtest: each subtest is a distinct goroutine,
			// and all use of a TaskService must stay on its creating goroutine.
			service, folder := maintenanceTestService(t)
			def := service.NewTaskDefinition()
			def.Settings.Enabled = false
			def.Settings.Compatibility = tt.compatibility
			def.Settings.MaintenanceSettings = tt.settings
			def.AddAction(ExecAction{Path: "cmd.exe", Args: "/c exit 0"})
			path := folder + `\RoundTrip`
			created, ok, err := service.CreateTask(path, def, false)
			if err != nil {
				t.Fatal(err)
			}
			defer created.Release()
			if !ok {
				t.Fatal("temporary task already exists")
			}
			assertMaintenanceTask(t, created, tt.settings)

			fetched, err := service.GetRegisteredTask(path)
			if err != nil {
				t.Fatal(err)
			}
			defer fetched.Release()
			assertMaintenanceTask(t, fetched, tt.settings)
		})
	}
}

func TestMaintenanceSettingsUpdate(t *testing.T) {
	service, folder := maintenanceTestService(t)
	def := service.NewTaskDefinition()
	def.Settings.Enabled = false
	def.Settings.Compatibility = TASK_COMPATIBILITY_V2_2
	def.Settings.MaintenanceSettings = &MaintenanceSettings{
		Period: period.NewYMD(0, 0, 2), Deadline: period.NewYMD(0, 0, 14), Exclusive: true,
	}
	def.AddAction(ExecAction{Path: "cmd.exe", Args: "/c exit 0"})
	path := folder + `\Update`
	created, ok, err := service.CreateTask(path, def, false)
	if err != nil {
		t.Fatal(err)
	}
	created.Release()
	if !ok {
		t.Fatal("temporary task already exists")
	}

	// Updating uses a fresh definition: remove an existing deadline, remove
	// maintenance entirely, then opt in again. No task is enabled or run.
	for _, maintenance := range []*MaintenanceSettings{
		{Period: period.NewYMD(0, 0, 1)},
		nil,
		{Period: period.NewYMD(0, 0, 3), Deadline: period.NewYMD(0, 0, 10)},
	} {
		def.Settings.MaintenanceSettings = maintenance
		updated, err := service.UpdateTask(path, def)
		if err != nil {
			t.Fatal(err)
		}
		updated.Release()
		fetched, err := service.GetRegisteredTask(path)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer fetched.Release()
			assertMaintenanceTask(t, fetched, maintenance)
		}()
	}
}

// Use a fresh folder for each integration test. Never reset the existing test
// suite's shared folder or enumerate/mutate unrelated user tasks.
func maintenanceTestService(t *testing.T) (*TaskService, string) {
	t.Helper()
	service, err := Connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Disconnect)
	folder := fmt.Sprintf(`\TaskmasterMaintenanceTests-%d-%d`, os.Getpid(), time.Now().UnixNano())
	v, err := oleutil.CallMethod(service.rootFolderObj, "CreateFolder", folder, "")
	if v != nil {
		v.Clear()
	}
	if err != nil {
		t.Fatalf("create temporary task folder: %v", err)
	}
	t.Cleanup(func() {
		if _, err := service.DeleteFolder(folder, true); err != nil {
			t.Errorf("delete temporary task folder %s: %v", folder, err)
		}
	})
	return &service, folder
}

func assertMaintenanceTask(t *testing.T, task RegisteredTask, want *MaintenanceSettings) {
	t.Helper()
	if task.Enabled || task.Definition.Settings.Enabled {
		t.Fatal("test task must remain disabled")
	}
	got := task.Definition.Settings.MaintenanceSettings
	var document struct {
		Settings struct {
			Unified     bool `xml:"UseUnifiedSchedulingEngine"`
			Maintenance *struct {
				Period    string  `xml:"Period"`
				Deadline  *string `xml:"Deadline"`
				Exclusive bool    `xml:"Exclusive"`
			} `xml:"MaintenanceSettings"`
		} `xml:"Settings"`
	}
	decoder := xml.NewDecoder(strings.NewReader(task.Definition.XMLText))
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		// The COM BSTR has already been converted to UTF-8 by go-ole, even if
		// the task XML still declares its original UTF-16 encoding.
		if strings.EqualFold(charset, "utf-16") {
			return input, nil
		}
		return nil, fmt.Errorf("unexpected task XML encoding %q", charset)
	}
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("parse persisted task XML: %v", err)
	}
	if want == nil {
		if got != nil || document.Settings.Maintenance != nil {
			t.Fatalf("ordinary task has unexpected maintenance settings: %+v", got)
		}
		return
	}
	if got == nil || document.Settings.Maintenance == nil {
		t.Fatal("maintenance settings were not persisted")
	}
	if !document.Settings.Unified {
		t.Fatal("maintenance task must enable UseUnifiedSchedulingEngine")
	}
	// These test periods contain whole days only; compare elapsed durations so
	// equivalent Windows serialization (P1D vs PT24H) does not fail the test.
	if got.Period.DurationApprox() != want.Period.DurationApprox() ||
		got.Deadline.DurationApprox() != want.Deadline.DurationApprox() || got.Exclusive != want.Exclusive {
		t.Fatalf("maintenance round trip: want %+v, got %+v", *want, *got)
	}
	if want.Deadline.IsZero() && document.Settings.Maintenance.Deadline != nil {
		t.Fatal("zero Deadline must omit the XML element")
	}
	if document.Settings.Maintenance.Exclusive != want.Exclusive {
		t.Fatal("Exclusive was not persisted in task XML")
	}
}
