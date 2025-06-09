package future

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// --- Helper functions ---
func makeSuccessFuture(val int, delay time.Duration) Future[int] {
	if delay == 0 {
		return NewValue(val)
	}
	return func(ctx context.Context) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(delay):
			return val, nil
		}
	}
}

func makeErrorFuture(err error, delay time.Duration) Future[int] {
	if delay == 0 {
		return NewError[int](err)
	}
	return func(ctx context.Context) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(delay):
			return 0, err
		}
	}
}

// --- Basic Future creation tests ---
func TestNewValue(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		f := NewValue(42)
		v, err := f(context.Background())
		assert.NilError(t, err)
		assert.Equal(t, v, 42)
	})

	t.Run("string", func(t *testing.T) {
		f := NewValue("hello")
		v, err := f(context.Background())
		assert.NilError(t, err)
		assert.Equal(t, v, "hello")
	})

	t.Run("struct", func(t *testing.T) {
		type S struct{ X int }
		f := NewValue(S{X: 7})
		v, err := f(context.Background())
		assert.NilError(t, err)
		assert.Equal(t, v.X, 7)
	})
}

// --- Await (multi-future) behavior tests ---

func TestAwait_AllSuccess(t *testing.T) {
	ctx := context.Background()
	num := 5
	futures := make([]Future[int], num)
	for i := range num {
		futures[i] = makeSuccessFuture(i, 10*time.Millisecond)
	}
	got := make(map[int]bool)
	count := 0
	seq := slices.Values(futures)
	for res := range Await(ctx, seq) {
		assert.NilError(t, res.Err)
		got[res.Value] = true
		count++
	}
	assert.Equal(t, count, num)
	for i := range num {
		assert.Assert(t, got[i], "missing value %d", i)
	}
}

func TestAwait_WithErrors(t *testing.T) {
	ctx := context.Background()
	errTest := errors.New("test error")
	futures := []Future[int]{
		makeSuccessFuture(1, 5*time.Millisecond),
		makeErrorFuture(errTest, 10*time.Millisecond),
		makeSuccessFuture(2, 15*time.Millisecond),
	}
	seq := slices.Values(futures)
	var gotVals []int
	var gotErrs []error
	for res := range Await(ctx, seq) {
		if res.Err != nil {
			gotErrs = append(gotErrs, res.Err)
		} else {
			gotVals = append(gotVals, res.Value)
		}
	}
	assert.Equal(t, len(gotVals), 2)
	assert.Equal(t, len(gotErrs), 1)
	assert.Assert(t, errors.Is(gotErrs[0], errTest), "expected error %v, got %v", errTest, gotErrs[0])
}

func TestAwait_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	num := 10
	futures := make([]Future[int], num)
	for i := range num {
		futures[i] = makeSuccessFuture(i, 50*time.Millisecond)
	}
	seq := slices.Values(futures)
	var count int32
	for range Await(ctx, seq) {
		atomic.AddInt32(&count, 1)
	}
	// Even if ctx is canceled, iteration continues and all results are eventually yielded (with error)
	assert.Equal(t, count, int32(num))
}

func TestAwait_Empty(t *testing.T) {
	ctx := context.Background()
	var futures []Future[int]
	seq := slices.Values(futures)
	called := false
	for range Await(ctx, seq) {
		called = true
	}
	assert.Assert(t, !called, "expected no results for empty input")
}

func TestAwait_ConcurrentOrder(t *testing.T) {
	ctx := context.Background()
	num := runtime.NumCPU() * 2
	futures := make([]Future[int], num)
	for i := range num {
		delay := time.Duration((num-i)*5) * time.Millisecond
		futures[i] = makeSuccessFuture(i, delay)
	}
	seq := slices.Values(futures)
	var got []int
	for res := range Await(ctx, seq) {
		assert.NilError(t, res.Err)
		got = append(got, res.Value)
	}
	assert.Equal(t, len(got), num)
	vals := make(map[int]bool)
	for _, v := range got {
		vals[v] = true
	}
	for i := range num {
		assert.Assert(t, vals[i], "missing value %d", i)
	}
}

func TestAwait_PanicFuture(t *testing.T) {
	ctx := context.Background()
	panicMsg := "panic in future"
	future := Future[int](func(_ context.Context) (int, error) {
		panic(panicMsg)
	})
	seq := slices.Values([]Future[int]{future})
	called := false
	for res := range Await(ctx, seq) {
		called = true
		assert.Assert(t, res.Err != nil && res.Err.Error() == panicMsg, "expected panic error %q, got %v", panicMsg, res.Err)
	}
	assert.Assert(t, called, "callback not called")
}

func TestAwait_Timeout_Multi(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	seq := slices.Values([]Future[int]{
		makeSuccessFuture(1, 100*time.Millisecond),
		makeSuccessFuture(2, 200*time.Millisecond),
		makeSuccessFuture(3, 300*time.Millisecond),
	})
	var countErr int
	for res := range Await(ctx, seq) {
		if res.Err != nil {
			countErr++
			assert.ErrorIs(t, res.Err, context.DeadlineExceeded)
		}
	}
	assert.Assert(t, countErr >= 1, "at least one result should have context.DeadlineExceeded")
}

// --- Future (single) method tests ---
func TestFuture_Await(t *testing.T) {
	ctx := context.Background()
	want := 42
	f := NewValue(want)
	got, err := f.Await(ctx)
	assert.NilError(t, err)
	assert.Equal(t, got, want)

	// Error case
	errTest := errors.New("test error")
	fErr := NewError[int](errTest)
	_, err = fErr.Await(ctx)
	assert.Assert(t, errors.Is(err, errTest), "expected error %v, got %v", errTest, err)
}

func TestFuture_Await_Timeout(t *testing.T) {
	f := makeSuccessFuture(1, 100*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := f.Await(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// --- future.New / future.NewDeferred tests ---
// These tests verify that New and NewDeferred always return the same result for all calls,
// and that the task is executed only once, even with concurrent calls.
func TestNewAndNewDeferred(t *testing.T) {
	types := []struct {
		name    string
		factory func(Task[int]) Future[int]
	}{
		{"New", func(task Task[int]) Future[int] { return New(context.Background(), task) }},
		{"NewDeferred", NewDeferred[int]},
	}

	for _, typ := range types {
		t.Run(typ.name+"/success", func(t *testing.T) {
			var count int32
			f := typ.factory(func(_ context.Context) (int, error) {
				atomic.AddInt32(&count, 1)
				return 42, nil
			})
			v, err := f(context.Background())
			assert.NilError(t, err)
			assert.Equal(t, v, 42)
			v, err = f(context.Background())
			assert.NilError(t, err)
			assert.Equal(t, v, 42)
			assert.Equal(t, atomic.LoadInt32(&count), int32(1))
		})

		t.Run(typ.name+"/error", func(t *testing.T) {
			var count int32
			errTest := errors.New("errTest")
			f := typ.factory(func(_ context.Context) (int, error) {
				atomic.AddInt32(&count, 1)
				return 0, errTest
			})
			_, err := f(context.Background())
			assert.ErrorIs(t, err, errTest)
			_, err = f(context.Background())
			assert.ErrorIs(t, err, errTest)
			assert.Equal(t, atomic.LoadInt32(&count), int32(1))
		})

		t.Run(typ.name+"/panic", func(t *testing.T) {
			var count int32
			panicMsg := "panic!"
			f := typ.factory(func(_ context.Context) (int, error) {
				atomic.AddInt32(&count, 1)
				panic(panicMsg)
			})
			_, err := f(context.Background())
			assert.Assert(t, err != nil)
			assert.Equal(t, err.Error(), panicMsg)
			_, err = f(context.Background())
			assert.Assert(t, err != nil)
			assert.Equal(t, err.Error(), panicMsg)
			assert.Equal(t, atomic.LoadInt32(&count), int32(1))
		})

		t.Run(typ.name+"/runs only once (concurrent)", func(t *testing.T) {
			var count int32
			f := typ.factory(func(_ context.Context) (int, error) {
				atomic.AddInt32(&count, 1)
				time.Sleep(10 * time.Millisecond)
				return 99, nil
			})
			var wg sync.WaitGroup
			n := 5
			wg.Add(n)
			results := make([]int, n)
			errs := make([]error, n)
			for i := range n {
				go func(i int) {
					defer wg.Done()
					results[i], errs[i] = f(context.Background())
				}(i)
			}
			wg.Wait()
			for i := range n {
				assert.NilError(t, errs[i])
				assert.Equal(t, results[i], 99)
			}
			assert.Equal(t, atomic.LoadInt32(&count), int32(1))
		})
	}
}
