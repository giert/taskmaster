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
	if err == nil {
		return nil
	}
	errCode, parseErr := getOLEErrorCode(err)
	if parseErr != nil {
		return err
	}

	// Task Scheduler errors surface either as a bare Win32 error code or as the
	// equivalent HRESULT (HRESULT_FROM_WIN32 -> 0x8007xxxx), so both forms are
	// handled here.
	switch errCode {
	case 2, 0x80070002: // ERROR_FILE_NOT_FOUND: the task does not exist
		return syscall.ERROR_FILE_NOT_FOUND // matches errors.Is(err, os.ErrNotExist)
	case 3, 0x80070003: // ERROR_PATH_NOT_FOUND: the task folder does not exist
		return syscall.ERROR_PATH_NOT_FOUND // matches errors.Is(err, os.ErrNotExist)
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
	if err == nil {
		return nil
	}
	errCode, parseErr := getOLEErrorCode(err)
	if parseErr != nil {
		return err
	}

	if errCode == 0x8004130B {
		return ErrRunningTaskCompleted
	}

	return syscall.Errno(errCode)
}

func getOLEErrorCode(err error) (uint32, error) {
	var oleErr *ole.OleError
	if !errors.As(err, &oleErr) || oleErr == nil {
		return 0, errors.New("failed to extract OLE error code")
	}

	// Invoke may provide a more specific error in EXCEPINFO. go-ole has already
	// cleared its native strings before constructing the OleError; reading the
	// error code must not free or otherwise mutate the caller's exception data.
	var code uint32
	switch excepInfo := oleErr.SubError().(type) {
	case ole.EXCEPINFO:
		code = excepInfo.SCODE()
	case *ole.EXCEPINFO:
		if excepInfo != nil {
			code = excepInfo.SCODE()
		}
	}
	if code != 0 {
		return code, nil
	}

	// A missing, unfamiliar, or empty EXCEPINFO does not invalidate the HRESULT
	// on the OleError itself. In particular, direct COM failures often have no
	// sub-error at all.
	return uint32(oleErr.Code()), nil
}
