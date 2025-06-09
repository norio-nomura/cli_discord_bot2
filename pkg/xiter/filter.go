// Package xiter provides adapters for Go 1.23+ iter.Seq, including Filter.
//
// This file contains Filter-related adapters.
package xiter

import (
	"iter"
)

// Filter returns a new iter.Seq[T] that yields only the elements of seq for which pred returns true.
func Filter[T any](seq iter.Seq[T], pred func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if pred(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}
