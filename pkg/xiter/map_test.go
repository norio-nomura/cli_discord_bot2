// Package xiter provides adapters for Go 1.23+ iter.Seq, including Map.
//
// This file contains tests for Map-related adapters.

package xiter

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

func TestMap_IntToString(t *testing.T) {
	seq := SeqOf(1, 2, 3)
	mapped := Map(seq, func(n int) string { return string(rune('A' + n - 1)) })
	got := slices.Collect(mapped)
	want := []string{"A", "B", "C"}
	assert.DeepEqual(t, got, want)
}

func TestMap_Empty(t *testing.T) {
	seq := SeqOf[int]()
	mapped := Map(seq, func(_ int) string { return "x" })
	got := slices.Collect(mapped)
	assert.Equal(t, len(got), 0)
}
