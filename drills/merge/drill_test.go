package merge

import (
	"testing"

	"gotutor/drills/internal/chantest"
)

func TestMerge(t *testing.T) { chantest.RunMergeTests(t, Merge[int]) }
