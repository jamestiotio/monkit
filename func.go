package monitor

import (
	"sync/atomic"
	"time"

	"github.com/bmizerany/perks/quantile"
	"github.com/spacemonkeygo/errors"
)

var (
	ObservedQuantiles = []float64{0, .25, .5, .75, .90, .95, .99, 1}
)

type Func struct {
	// sync/atomic things
	current int64
	success int64
	panics  int64
	parents funcSet

	// constructor things
	Id           int64
	Scope        *Scope
	Name         string
	SuccessTimes *quantile.Stream
	FailureTimes *quantile.Stream

	// mutex things (reusing the parents mutex)
	errors map[string]int64
}

func newFunc(s *Scope, name string) *Func {
	return &Func{
		Id:           newId(),
		Scope:        s,
		Name:         name,
		errors:       make(map[string]int64),
		SuccessTimes: quantile.NewTargeted(ObservedQuantiles...),
		FailureTimes: quantile.NewTargeted(ObservedQuantiles...),
	}
}

func (f *Func) start(parent *Func) {
	f.parents.Add(parent)
	atomic.AddInt64(&f.current, 1)
}

func (f *Func) end(errptr *error, panicked bool, duration time.Duration) {
	atomic.AddInt64(&f.current, -1)
	if panicked {
		atomic.AddInt64(&f.panics, 1)
		f.FailureTimes.Insert(duration.Seconds())
		return
	}
	if errptr == nil || *errptr == nil {
		atomic.AddInt64(&f.success, 1)
		f.SuccessTimes.Insert(duration.Seconds())
		return
	}
	f.FailureTimes.Insert(duration.Seconds())
	f.parents.Lock()
	f.errors[errors.GetClass(*errptr).String()] += 1
	f.parents.Unlock()
}

func (f *Func) Current() int64 { return atomic.LoadInt64(&f.current) }
func (f *Func) Success() int64 { return atomic.LoadInt64(&f.success) }
func (f *Func) Panics() int64  { return atomic.LoadInt64(&f.panics) }

func (f *Func) Errors() (rv map[string]int64) {
	f.parents.Lock()
	rv = make(map[string]int64, len(f.errors))
	for errname, count := range f.errors {
		rv[errname] = count
	}
	f.parents.Unlock()
	return rv
}

func (f *Func) Parents(cb func(f *Func)) {
	f.parents.Iterate(cb)
}
