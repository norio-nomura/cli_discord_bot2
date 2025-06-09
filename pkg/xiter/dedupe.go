// Package xiter provides adapters for Go 1.23+ iter.Seq, including Dedupe.
//
// This file contains Dedupe, which removes consecutive duplicates from a sequence.
package xiter

import "iter"

// Dedupe returns a new iter.Seq[T] that yields only the first occurrence of each unique value in seq.
// Values are considered duplicates if they are equal (==). Requires T to be comparable.
func Dedupe[T comparable](seq iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		seen := make(map[T]struct{})
		for v := range seq {
			if _, exists := seen[v]; !exists {
				seen[v] = struct{}{}
				if !yield(v) {
					return
				}
			}
		}
	}
}
