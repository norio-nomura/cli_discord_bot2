package xiter

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFilter_Int(t *testing.T) {
	seq := SeqOf(1, 2, 3, 4, 5, 6)
	isEven := func(n int) bool { return n%2 == 0 }
	filtered := Filter(seq, isEven)
	got := slices.Collect(filtered)
	want := []int{2, 4, 6}
	assert.DeepEqual(t, got, want)
}

func TestFilter_Empty(t *testing.T) {
	seq := SeqOf[int]()
	filtered := Filter(seq, func(n int) bool { return n%2 == 0 })
	count := 0
	for range filtered {
		count++
	}
	assert.Equal(t, count, 0)
}

func TestFilter_AllFalse(t *testing.T) {
	seq := SeqOf(1, 3, 5)
	filtered := Filter(seq, func(_ int) bool { return false })
	count := 0
	for range filtered {
		count++
	}
	assert.Equal(t, count, 0)
}

func TestFilter_AllTrue(t *testing.T) {
	seq := SeqOf(1, 2, 3)
	filtered := Filter(seq, func(_ int) bool { return true })
	got := slices.Collect(filtered)
	want := []int{1, 2, 3}
	assert.DeepEqual(t, got, want)
}
