// Package xiter provides adapters for Go 1.23+ iter.Seq, including Zip and ZipLongest.
//
// This file contains Zip-related adapters.
package xiter

import (
	"iter"
)

// Zipped holds a pair of values and their presence flags.
type Zipped[T, U any] struct {
	V1  T
	OK1 bool
	V2  U
	OK2 bool
}

// Zip takes two sequences and returns a new sequence that yields Zipped elements from the two sequences.
// If either sequence is exhausted, iteration stops (shortest sequence wins).
func Zip[T, U any](seqT iter.Seq[T], seqU iter.Seq[U]) iter.Seq[Zipped[T, U]] {
	return func(yield func(Zipped[T, U]) bool) {
		uNext, uStop := iter.Pull(seqU)
		defer uStop()
		for t := range seqT {
			if u, ok := uNext(); ok {
				if !yield(Zipped[T, U]{V1: t, OK1: true, V2: u, OK2: true}) {
					break
				}
			} else {
				break
			}
		}
	}
}

// ZipLongest takes two sequences and returns a new sequence that yields pairs of elements from the two sequences.
// If one sequence is shorter, the missing values are replaced with zero values and OK flags are set to false.
func ZipLongest[T, U any](seqT iter.Seq[T], seqU iter.Seq[U]) iter.Seq[Zipped[T, U]] {
	return func(yield func(Zipped[T, U]) bool) {
		tNext, tStop := iter.Pull(seqT)
		defer tStop()
		uNext, uStop := iter.Pull(seqU)
		defer uStop()
		for {
			t, okT := tNext()
			u, okU := uNext()
			if !okT && !okU {
				break
			}
			if !yield(Zipped[T, U]{V1: t, OK1: okT, V2: u, OK2: okU}) {
				break
			}
		}
	}
}
