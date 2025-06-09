package xiter

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

func TestZip_BothEmpty(t *testing.T) {
	seqA := SeqOf[int]()
	seqB := SeqOf[string]()
	got := slices.Collect(Zip(seqA, seqB))
	assert.Equal(t, len(got), 0)
}

func TestZip_LeftEmpty(t *testing.T) {
	seqA := SeqOf[int]()
	seqB := SeqOf("a", "b")
	got := slices.Collect(Zip(seqA, seqB))
	assert.Equal(t, len(got), 0)
}

func TestZip_RightEmpty(t *testing.T) {
	seqA := SeqOf(1, 2)
	seqB := SeqOf[string]()
	got := slices.Collect(Zip(seqA, seqB))
	assert.Equal(t, len(got), 0)
}

func TestZip_Normal(t *testing.T) {
	seqA := SeqOf(1, 2, 3)
	seqB := SeqOf("a", "b", "c", "d")
	got := slices.Collect(Zip(seqA, seqB))
	want := []Zipped[int, string]{
		{V1: 1, OK1: true, V2: "a", OK2: true},
		{V1: 2, OK1: true, V2: "b", OK2: true},
		{V1: 3, OK1: true, V2: "c", OK2: true},
	}
	assert.DeepEqual(t, got, want)
}

func TestZipLongest_Normal(t *testing.T) {
	seqA := SeqOf(1, 2)
	seqB := SeqOf("a", "b", "c")
	got := slices.Collect(ZipLongest(seqA, seqB))
	want := []Zipped[int, string]{
		{V1: 1, OK1: true, V2: "a", OK2: true},
		{V1: 2, OK1: true, V2: "b", OK2: true},
		{V1: 0, OK1: false, V2: "c", OK2: true},
	}
	assert.DeepEqual(t, got, want)
}

func TestZipLongest_BothEmpty(t *testing.T) {
	seqA := SeqOf[int]()
	seqB := SeqOf[string]()
	got := slices.Collect(ZipLongest(seqA, seqB))
	assert.Equal(t, len(got), 0)
}

func TestZipLongest_LeftLonger(t *testing.T) {
	seqA := SeqOf(1, 2, 3)
	seqB := SeqOf("a")
	got := slices.Collect(ZipLongest(seqA, seqB))
	want := []Zipped[int, string]{
		{V1: 1, OK1: true, V2: "a", OK2: true},
		{V1: 2, OK1: true, V2: "", OK2: false},
		{V1: 3, OK1: true, V2: "", OK2: false},
	}
	assert.DeepEqual(t, got, want)
}
