package xiter

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

// ステートレス性を確認するためのテスト
func TestDedupe_Stateless(t *testing.T) {
	seq := slices.Values([]int{1, 1, 2, 2, 3, 1, 1, 4})
	d := Dedupe(seq)
	got1 := slices.Collect(d)
	got2 := slices.Collect(d)
	want := []int{1, 2, 3, 4}
	assert.DeepEqual(t, got1, want)
	assert.DeepEqual(t, got2, want)
}

func TestFilter_Stateless(t *testing.T) {
	seq := slices.Values([]int{1, 2, 3, 4})
	isEven := func(n int) bool { return n%2 == 0 }
	f := Filter(seq, isEven)
	got1 := slices.Collect(f)
	got2 := slices.Collect(f) // 2回目も同じ結果になること
	want := []int{2, 4}
	assert.DeepEqual(t, got1, want)
	assert.DeepEqual(t, got2, want)
}

func TestMap_Stateless(t *testing.T) {
	seq := slices.Values([]int{1, 2, 3})
	f := Map(seq, func(n int) int { return n * 2 })
	got1 := slices.Collect(f)
	got2 := slices.Collect(f)
	want := []int{2, 4, 6}
	assert.DeepEqual(t, got1, want)
	assert.DeepEqual(t, got2, want)
}

func TestSeqOf_Stateless(t *testing.T) {
	seq := SeqOf(1, 2, 3)
	got1 := slices.Collect(seq)
	got2 := slices.Collect(seq)
	want := []int{1, 2, 3}
	assert.DeepEqual(t, got1, want)
	assert.DeepEqual(t, got2, want)
}

func TestZip_Stateless(t *testing.T) {
	seqA := slices.Values([]int{1, 2, 3})
	seqB := slices.Values([]string{"a", "b", "c"})
	z := Zip(seqA, seqB)
	collect := func() []Zipped[int, string] {
		return slices.Collect(z)
	}
	got1 := collect()
	got2 := collect()
	want := []Zipped[int, string]{
		{V1: 1, OK1: true, V2: "a", OK2: true},
		{V1: 2, OK1: true, V2: "b", OK2: true},
		{V1: 3, OK1: true, V2: "c", OK2: true},
	}
	assert.DeepEqual(t, got1, want)
	assert.DeepEqual(t, got2, want)
}

func TestZipLongest_Stateless(t *testing.T) {
	seqA := slices.Values([]int{1, 2})
	seqB := slices.Values([]string{"a", "b", "c"})
	z := ZipLongest(seqA, seqB)
	collect := func() []Zipped[int, string] {
		return slices.Collect(z)
	}
	got1 := collect()
	got2 := collect()
	if !slices.Equal(got1, got2) {
		t.Errorf("ZipLongest is not stateless: got1=%v, got2=%v", got1, got2)
	}
}
