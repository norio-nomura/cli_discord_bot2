// Package xiter provides adapters for Go 1.23+ iter.Seq, including SeqOf.
//
// This file contains SeqOf, which creates a sequence from variadic arguments.
package xiter

import "iter"

// SeqOf returns an iter.Seq[T] that yields all the given values in order.
func SeqOf[T any](vals ...T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range vals {
			if !yield(v) {
				return
			}
		}
	}
}
