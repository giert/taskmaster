//go:build windows
// +build windows

package taskmaster

import (
	"math"
	"testing"
	"time"

	ole "github.com/go-ole/go-ole"
)

func TestVariantTimeOrZero(t *testing.T) {
	if got := variantTimeOrZero(nil); !got.IsZero() {
		t.Fatalf("expected zero time for nil variant, got %v", got)
	}

	if got := variantTimeOrZero(&ole.VARIANT{VT: ole.VT_I4, Val: 10}); !got.IsZero() {
		t.Fatalf("expected zero time for non-date variant, got %v", got)
	}

	vtDate := &ole.VARIANT{VT: ole.VT_DATE, Val: int64(math.Float64bits(2.5))}
	got := variantTimeOrZero(vtDate)
	expected := time.Date(1900, time.January, 1, 12, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}
