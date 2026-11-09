//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

	ole "github.com/go-ole/go-ole"
)

func TestGetOLEErrorCodeFallsBackToHRESULT(t *testing.T) {
	const code = uint32(0x80070002)
	var nilException *ole.EXCEPINFO
	tests := []struct {
		name string
		err  error
	}{
		{name: "no sub-error", err: ole.NewError(uintptr(code))},
		{name: "description only", err: ole.NewErrorWithDescription(uintptr(code), "missing task")},
		{name: "wrapped", err: fmt.Errorf("looking up task: %w", ole.NewError(uintptr(code)))},
		{name: "unrecognized sub-error", err: ole.NewErrorWithSubError(uintptr(code), "", errors.New("details"))},
		{name: "empty exception value", err: ole.NewErrorWithSubError(uintptr(code), "", ole.EXCEPINFO{})},
		{name: "empty exception pointer", err: ole.NewErrorWithSubError(uintptr(code), "", &ole.EXCEPINFO{})},
		{name: "nil exception pointer", err: ole.NewErrorWithSubError(uintptr(code), "", nilException)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getOLEErrorCode(tt.err)
			if err != nil || got != code {
				t.Fatalf("want HRESULT %#x, got %#x, error %v", code, got, err)
			}
		})
	}
}

func TestTaskSchedulerErrorMappings(t *testing.T) {
	tests := []struct {
		name string
		code uintptr
		want error
	}{
		{name: "raw file not found", code: 2, want: os.ErrNotExist},
		{name: "file not found HRESULT", code: 0x80070002, want: os.ErrNotExist},
		{name: "raw path not found", code: 3, want: os.ErrNotExist},
		{name: "path not found HRESULT", code: 0x80070003, want: os.ErrNotExist},
		{name: "unsupported target", code: 50, want: ErrTargetUnsupported},
		{name: "raw bad network path", code: 53, want: ErrConnectionFailure},
		{name: "bad network path HRESULT", code: 0x80070035, want: ErrConnectionFailure},
		{name: "other HRESULT preserved", code: 0x80070005, want: syscall.Errno(0x80070005)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fmt.Errorf("COM operation: %w", ole.NewError(tt.code))
			got := getTaskSchedulerError(err)
			if !errors.Is(got, tt.want) {
				t.Fatalf("HRESULT %#x: want %v, got %v", tt.code, tt.want, got)
			}
		})
	}
}

func TestRunningTaskErrorMappings(t *testing.T) {
	err := fmt.Errorf("refresh: %w", ole.NewError(0x8004130B))
	if got := getRunningTaskError(err); !errors.Is(got, ErrRunningTaskCompleted) {
		t.Fatalf("want completed-task sentinel, got %v", got)
	}
	if got := getRunningTaskError(ole.NewError(0x80070005)); !errors.Is(got, syscall.Errno(0x80070005)) {
		t.Fatalf("want original HRESULT, got %v", got)
	}
}

func TestErrorConversionPreservesNonOLEErrors(t *testing.T) {
	sentinel := errors.New("original failure")
	wrapped := fmt.Errorf("context: %w", sentinel)
	var nilOLE *ole.OleError
	for name, convert := range map[string]func(error) error{
		"scheduler":    getTaskSchedulerError,
		"running task": getRunningTaskError,
	} {
		t.Run(name, func(t *testing.T) {
			for _, input := range []error{nil, sentinel, wrapped, syscall.ERROR_FILE_NOT_FOUND, nilOLE} {
				if got := convert(input); got != input {
					t.Fatalf("conversion replaced original error: want %v, got %v", input, got)
				}
			}
		})
	}
}
