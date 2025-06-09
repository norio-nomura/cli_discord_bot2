package future

import (
	"context"
	"fmt"
	"iter"
	"runtime"
	"slices"
	"sync"
)

// Task is a function type that performs an asynchronous computation and returns a value of type T or an error.
// It is intended to be executed once to produce a result.
type Task[T any] func(context.Context) (T, error)

// Future is a function type that returns the result (value or error) of a Task.
// It always returns the same result for all calls, regardless of how many times it is invoked.
type Future[T any] func(context.Context) (T, error)

// New creates a Future from a Task and starts asynchronous execution immediately.
// The returned Future will return the result (value or error) when called, and always returns the same result for all calls.
func New[T any](ctx context.Context, task Task[T]) Future[T] {
	runner, receiver := makeRunnerAndReceiver(task)
	go runner(ctx)
	return receiver
}

// NewDeferred creates a Future from a Task.
// The returned Future will start asynchronous execution the first time it is called, and always returns the same result for all calls.
func NewDeferred[T any](task Task[T]) Future[T] {
	runner, receiver := makeRunnerAndReceiver(task)
	var once sync.Once
	return func(ctx context.Context) (T, error) {
		once.Do(func() { go runner(ctx) })
		return receiver(ctx)
	}
}

// NewValue returns a Future that always resolves to the given value.
func NewValue[T any](v T) Future[T] {
	return func(_ context.Context) (T, error) {
		return v, nil
	}
}

// NewError returns a Future that always fails with the given error.
func NewError[T any](err error) Future[T] {
	var zero T
	return func(_ context.Context) (T, error) {
		return zero, err
	}
}

// Await waits for the Future to complete and returns the value or error. Equivalent to calling f(ctx).
func (f Future[T]) Await(ctx context.Context) (T, error) {
	return f(ctx)
}

// Result holds the result (value or error) of a Future execution.
type Result[T any] struct {
	Value T
	Err   error
}

func makeRunnerAndReceiver[T any](task func(context.Context) (T, error)) (runner func(context.Context), receiver func(context.Context) (T, error)) {
	ch := make(chan Result[T])
	runner = func(ctx context.Context) {
		defer close(ch)
		var result Result[T]
		defer func() { ch <- result }()
		defer result.recover()
		result.Value, result.Err = task(ctx)
	}
	var once sync.Once
	var result Result[T]
	receiver = func(ctx context.Context) (T, error) {
		once.Do(func() {
			result.receive(ctx, ch)
		})
		return result.Value, result.Err
	}
	return runner, receiver
}

// receive receives a value or error from the Future's result channel.
// If ctx is canceled, ctx.Err() is returned unless the result is already available.
func (r *Result[T]) receive(ctx context.Context, ch <-chan Result[T]) {
	select {
	case result, ok := <-ch:
		if ok {
			*r = result
		}
	case <-ctx.Done():
		select {
		case result, ok := <-ch: // read from channel if result is available
			if ok {
				*r = result
			}
		default:
			r.Err = ctx.Err()
		}
	}
}

// recover recovers from a panic in a goroutine and stores it as an error.
func (r *Result[T]) recover() {
	switch v := recover().(type) {
	case nil:
	case error:
		r.Err = v
	default:
		r.Err = fmt.Errorf("%+v", v)
	}
}

// Await runs multiple Futures in parallel and yields their results (value or error) as they complete.
// Even if ctx is canceled, all results are eventually yielded (with error) for each Future.
func Await[T any](ctx context.Context, futures iter.Seq[Future[T]]) iter.Seq[Result[T]] {
	receiverCh := make(chan func(context.Context) (T, error), runtime.NumCPU())
	go func() {
		defer close(receiverCh)
		for f := range futures {
			runner, receiver := makeRunnerAndReceiver(f)
			receiverCh <- receiver
			go runner(ctx)
		}
	}()
	return slices.Values(slices.Collect(func(yield func(Result[T]) bool) {
		for receiver := range receiverCh {
			var result Result[T]
			result.Value, result.Err = receiver(ctx)
			if !yield(result) {
				return
			}
		}
	}))
}
