package xiter

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSeqOf_Int(t *testing.T) {
	seq := SeqOf(1, 2, 3, 4)
	got := slices.Collect(seq)
	want := []int{1, 2, 3, 4}
	assert.DeepEqual(t, got, want)
}

func TestSeqOf_String(t *testing.T) {
	seq := SeqOf("a", "b", "c")
	got := slices.Collect(seq)
	want := []string{"a", "b", "c"}
	assert.DeepEqual(t, got, want)
}

func TestSeqOf_Empty(t *testing.T) {
	seq := SeqOf[int]()
	got := slices.Collect(seq)
	assert.Equal(t, len(got), 0)
}
