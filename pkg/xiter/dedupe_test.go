package xiter

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestDedupe_Int(t *testing.T) {
	input := SeqOf(1, 1, 2, 2, 2, 3, 1, 1, 4)
	got := make([]int, 0)
	for v := range Dedupe(input) {
		got = append(got, v)
	}
	want := []int{1, 2, 3, 4}
	assert.DeepEqual(t, got, want)
}

func TestDedupe_String(t *testing.T) {
	input := SeqOf("a", "a", "b", "b", "a", "a", "c")
	got := make([]string, 0)
	for v := range Dedupe(input) {
		got = append(got, v)
	}
	want := []string{"a", "b", "c"}
	assert.DeepEqual(t, got, want)
}
