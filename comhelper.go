//go:build windows
// +build windows

package taskmaster

import (
	"fmt"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// oleHelper performs a batch of COM property reads/writes against Task Scheduler
// objects, recording the first error encountered. Once an error is recorded,
// every subsequent operation becomes a no-op returning the zero value, so a long
// sequence of calls can be issued and the accumulated error checked once at a
// convenient point via h.err.
//
// This replaces the panic-on-error oleutil.Must* helpers with ordinary error propagation
//
// IMPORTANT: getObject and query return nil once an error has been recorded.
// Callers MUST check h.err immediately after such a call, before dereferencing or
// deferring Release on the result, e.g.:
//
//	obj := h.getObject(parent, "Child")
//	if h.err != nil {
//		return h.err
//	}
//	defer obj.Release()
type oleHelper struct {
	err error
}

func (h *oleHelper) getString(obj *ole.IDispatch, name string) string {
	if h.err != nil {
		return ""
	}
	v, err := oleutil.GetProperty(obj, name)
	if err != nil {
		h.err = fmt.Errorf("error getting property %q: %w", name, err)
		return ""
	}
	defer v.Clear()
	return v.ToString()
}

func (h *oleHelper) getInt(obj *ole.IDispatch, name string) int64 {
	if h.err != nil {
		return 0
	}
	v, err := oleutil.GetProperty(obj, name)
	if err != nil {
		h.err = fmt.Errorf("error getting property %q: %w", name, err)
		return 0
	}
	defer v.Clear()
	return v.Val
}

func (h *oleHelper) getBool(obj *ole.IDispatch, name string) bool {
	if h.err != nil {
		return false
	}
	v, err := oleutil.GetProperty(obj, name)
	if err != nil {
		h.err = fmt.Errorf("error getting property %q: %w", name, err)
		return false
	}
	defer v.Clear()
	b, ok := v.Value().(bool)
	if !ok {
		h.err = fmt.Errorf("property %q is not a boolean", name)
		return false
	}
	return b
}

func (h *oleHelper) getVariant(obj *ole.IDispatch, name string) *ole.VARIANT {
	if h.err != nil {
		return nil
	}
	v, err := oleutil.GetProperty(obj, name)
	if err != nil {
		h.err = fmt.Errorf("error getting property %q: %w", name, err)
		return nil
	}
	return v
}

func (h *oleHelper) getObject(obj *ole.IDispatch, name string) *ole.IDispatch {
	if h.err != nil {
		return nil
	}
	v, err := oleutil.GetProperty(obj, name)
	if err != nil {
		h.err = fmt.Errorf("error getting property %q: %w", name, err)
		return nil
	}
	d := v.ToIDispatch()
	if d == nil {
		h.err = fmt.Errorf("property %q is not an object", name)
		return nil
	}
	return d
}

func (h *oleHelper) put(obj *ole.IDispatch, name string, args ...any) {
	if h.err != nil {
		return
	}
	if _, err := oleutil.PutProperty(obj, name, args...); err != nil {
		h.err = fmt.Errorf("error setting property %q: %w", name, err)
	}
}

func (h *oleHelper) query(obj *ole.IDispatch, iid *ole.GUID) *ole.IDispatch {
	if h.err != nil {
		return nil
	}
	d, err := obj.QueryInterface(iid)
	if err != nil {
		h.err = fmt.Errorf("error querying COM interface: %w", err)
		return nil
	}
	return d
}
