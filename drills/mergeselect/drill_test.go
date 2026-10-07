package mergeselect

import (
	"testing"

	"gotutor/drills/internal/chantest"
)

func TestMergeSelect(t *testing.T) { chantest.RunMergeTests(t, MergeSelect[int]) }

func TestExactlyOneSelect(t *testing.T) {
	if n := chantest.CountSelects(t, "drill.go", "MergeSelect"); n != 1 {
		t.Errorf("MergeSelect has %d select statements; want exactly 1", n)
	}
}
