//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"syscall"

	ole "github.com/go-ole/go-ole"
)

var (
	ErrTargetUnsupported    = errors.New("error connecting to the Task Scheduler service: cannot connect to the XP or server 2003 computer")
	ErrConnectionFailure    = errors.New("error connecting to the Task Scheduler service: cannot connect to target computer")
	ErrInvalidPath          = errors.New(`path must start with root folder "\"`)
	ErrNoActions            = errors.New("definition must have at least one action")
	ErrInvalidPrincipal     = errors.New("both UserId and GroupId are defined for the principal; they are mutually exclusive")
	ErrRunningTaskCompleted = errors.New("the running task completed while it was getting parsed")
)

func getTaskSchedulerError(err error) error {
	errCode, parseErr := getOLEErrorCode(err)
	if parseErr != nil {
		return parseErr
	}

	// Task Scheduler connection failures surface either as a bare Win32 error
	// code or as the equivalent HRESULT (HRESULT_FROM_WIN32 -> 0x8007xxxx), so
	// both forms are handled here.
	switch errCode {
	case 50: // ERROR_NOT_SUPPORTED: target is an unsupported OS (e.g. XP / Server 2003)
		return ErrTargetUnsupported
	case 53, // ERROR_BAD_NETPATH (raw)
		0x80070035, // HRESULT_FROM_WIN32(ERROR_BAD_NETPATH)
		0x80070032: // observed when the remote Task Scheduler cannot be reached
		return ErrConnectionFailure
	default:
		return syscall.Errno(errCode)
	}
}

func getRunningTaskError(err error) error {
	errCode, parseErr := getOLEErrorCode(err)
	if parseErr != nil {
		return parseErr
	}

	if errCode == 0x8004130B {
		return ErrRunningTaskCompleted
	}

	return syscall.Errno(errCode)
}

func getOLEErrorCode(err error) (uint32, error) {
	if oleErr, ok1 := err.(*ole.OleError); ok1 {
		if excepInfo, ok2 := oleErr.SubError().(ole.EXCEPINFO); ok2 {
			return excepInfo.SCODE(), nil
		} else {
			return uint32(oleErr.Code()), errors.New("failed to extract OLE sub-error code")
		}
	}
	return 0, errors.New("failed to extract OLE error code")
}
