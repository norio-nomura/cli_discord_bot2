// Package xiter provides adapters for Go 1.23+ iter.Seq, including Map.
//
// This file contains Map-related adapters.
package xiter

import "iter"

// Map returns a new iter.Seq[U] that yields f(v) for each v in seq.
func Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}
