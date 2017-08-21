// Package meteredwriter provides tools to combine io.Writer and
// metrics.Histogram interfaces, so that every non-empty Write call latency
// value is sampled to Histogram.
//
// MeteredWriter can be used to bind standard metrics.Histogram to io.Writer.
// SelfCleaningHistogram provides a wrapper over metrics.Histogram with
// self-cleaning capabilities, which can be used for sharing one Histogram over
// multiple io.Writers and cleaning sample pool after period of inactivity.
//
// This package is intended to be used with go-metrics:
// (https://github.com/rcrowley/go-metrics) or metrics
// (https://github.com/facebookgo/metrics) packages.
package mio

import (
	"sync/atomic"
	"time"
)

// Histogram interface wraps a subset of methods of metrics.Histogram interface
// so it can be used without type conversion.
type Histogram interface {
	Clear()
	Count() int64
	Max() int64
	Mean() float64
	Min() int64
	Percentile(float64) float64
	Percentiles([]float64) []float64
	StdDev() float64
	Update(int64)
	Variance() float64
}

// SelfCleaningHistogram wraps metrics.Histogram, adding self-cleaning feature
// if no samples were registered for a specified time. SelfCleaningHistogram
// also implements Registrar interface, call Register() method to announce
// following sample updates, call Done() after all samples were added. If no
// outstanding workers registered (for each Register() call Done() call were
// made), self-cleaning timer would start, cleaning histogram's sample pool in
// absence of Register() calls before timer fires.
type SelfCleaningHistogram struct {
	Histogram
	c, q, d chan struct{}
	closed  bool
	cnt     uint64
}

// Registrar interface can be used to track object's concurrent usage.
//
// Its Register method announces goroutine's intent to use this object's
// facilities; Done method should be called when goroutine finished working with
// this object. Shutdown method stops associated background goroutines so that
// resources can be garbage collected.
//
// These methods provide a similar semantics as sync.WaitGroup's Add(1), and
// Done() methods.
type Registrar interface {
	Register()
	Done()
	Shutdown()
}

// NewSelfCleaningHistogram returns SelfCleaningHistogram wrapping specified
// histogram; its self-cleaning period set to delay.
func NewSelfCleaningHistogram(histogram Histogram, delay time.Duration) *SelfCleaningHistogram {
	h := &SelfCleaningHistogram{
		Histogram: histogram,
		c:         make(chan struct{}),
		q:         make(chan struct{}),
		d:         make(chan struct{}, 1),
	}
	// make sure goroutine is started before returning
	guard := make(chan struct{})
	go h.decay(delay, guard)
	<-guard
	return h
}

// decay tracks usage of SelfCleaningHistogram, starting and stopping cleaning
// timer as needed
func (h *SelfCleaningHistogram) decay(delay time.Duration, guard chan<- struct{}) {
	var t *time.Timer
	close(guard)
	for {
		select {
		case <-h.c:
		case <-h.q:
			if t != nil {
				t.Stop()
			}
			return
		}
		if t != nil {
			t.Stop()
		}
		<-h.d
		t = time.AfterFunc(delay, h.Clear)
	}
}

// Register implements Registrar interface, using sync.WaitGroup.Add(1) for each
// call, blocking self-cleaning timer until all object's users releases it with
// Done() call.
func (h *SelfCleaningHistogram) Register() {
	atomic.AddUint64(&h.cnt, 1)
	select {
	case h.c <- struct{}{}:
	default:
	}
}

// Done implements Registrar interface, using sync.WaitGroup.Done() for each
// call.
func (h *SelfCleaningHistogram) Done() {
	cnt := atomic.AddUint64(&h.cnt, ^uint64(0))
	if cnt == 0 {
		select {
		case h.d <- struct{}{}:
		default:
		}
	}
}

// Shutdown implements Registrar interface, it stops background goroutine. This
// method should be called as the very last method on object and needed only if
// object has to be removed and garbage collected.
func (h *SelfCleaningHistogram) Shutdown() {
	if !h.closed {
		h.closed = true
		close(h.q)
	}
}
