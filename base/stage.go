package base

import (
	"sync"
)

// Stage represents a data-staging "pipeline breaker", that's
// processed by one or more concurrent actors.
type Stage struct {
	Vars *Vars

	YieldVals  YieldVals
	YieldStats YieldStats
	YieldErr   YieldErr

	BatchCh chan []Vals

	M sync.Mutex // Protects the fields that follow.

	NumActors int // Inc'ed during StageStartActor().

	StopCh chan struct{} // When error, close()'ed and nil'ed.

	Err error

	Recycled [][]Vals
}

func NewStage(batchChSize int, vars *Vars,
	yieldVals YieldVals, yieldStats YieldStats, yieldErr YieldErr) *Stage {
	return &Stage{
		Vars:       vars,
		YieldVals:  yieldVals,
		YieldStats: yieldStats,
		YieldErr:   yieldErr,

		BatchCh: make(chan []Vals, batchChSize),

		StopCh: make(chan struct{}),
	}
}

type ActorFunc func(*Vars, YieldVals, YieldStats, YieldErr, interface{})

// StageStartActor is used for data-staging and "pipeline breaking"
// and spawns a concurrent actor (goroutine) related to the given
// stage.  A batchSize > 0 means there will be batching of results.  A
// batchSize of 1, for example, means send each incoming result as its
// own batch-of-1.  A batchSize of <= 0 means an actor will send a
// single, giant batch at the end.
func (stage *Stage) StartActor(aFunc ActorFunc, aData interface{}, batchSize int) {
	stage.M.Lock()

	stage.NumActors++

	stopCh := stage.StopCh // Own copy for reading.

	stage.M.Unlock()

	var err error

	var batch []Vals

	batchSend := func() {
		if len(batch) > 0 {
			select {
			case <-stopCh: // Sibling actor had an error.
				stage.M.Lock()
				if err == nil {
					err = stage.Err
				}
				stage.M.Unlock()

			case stage.BatchCh <- batch:
				// NO-OP.
			}

			batch = nil
		}
	}

	yieldVals := func(vals Vals) {
		if err == nil {
			// Need to materialize or deep-copy the incoming vals into
			// the batch, so reuse slices from previously recycled
			// batch, if any.
			if batch == nil {
				batch = stage.AcquireBatch()[:0]
			}

			var preallocVals Vals
			var preallocVal Val

			if cap(batch) > len(batch) {
				preallocVals := batch[0 : len(batch)+1][len(batch)]
				preallocVals = preallocVals[0:cap(preallocVals)]

				if len(preallocVals) > 0 {
					preallocVal = preallocVals[0]
					preallocVal = preallocVal[0:cap(preallocVal)]
				}
			}

			valsCopy, _, _ := ValsDeepCopy(vals, preallocVals, preallocVal)

			batch = append(batch, valsCopy)

			if batchSize > 0 {
				if len(batch) >= batchSize {
					batchSend()
				}
			}
		}
	}

	yieldErr := func(errIn error) {
		if errIn != nil {
			err = errIn

			stage.M.Lock()

			if stage.Err == nil {
				stage.Err = errIn // First error by any actor.

				// Closed & nil'ed under lock to have single close().
				if stage.StopCh != nil {
					close(stage.StopCh)
					stage.StopCh = nil
				}
			}

			stage.M.Unlock()
		}

		if err == nil {
			batchSend() // Send the last, in-flight batch.
		}
	}

	go func() {
		if stopCh != nil {
			aFunc(stage.Vars, yieldVals, stage.YieldStats, yieldErr, aData)
		}

		stage.BatchCh <- nil // Must send last nil, meaning this actor is done.
	}()
}

// --------------------------------------------------------

func (stage *Stage) WaitForActors() {
	stage.M.Lock()
	numActors := stage.NumActors
	stage.M.Unlock()

	var numActorsDone int

	for numActorsDone < numActors {
		batch := <-stage.BatchCh
		if batch == nil {
			numActorsDone++
		} else {
			for _, vals := range batch {
				stage.YieldVals(vals)
			}

			stage.RecycleBatch(batch)
		}
	}

	stage.M.Lock()

	stage.YieldErr(stage.Err)

	stage.M.Unlock()
}

// --------------------------------------------------------

// RecycleBatch holds onto a batch for a future AcquireBatch().
func (stage *Stage) RecycleBatch(batch []Vals) {
	stage.M.Lock()
	stage.Recycled = append(stage.Recycled, batch)
	stage.M.Unlock()
}

// AcquireBatch returns either a previously recycled batch or nil if
// there aren't any.
func (stage *Stage) AcquireBatch() (rv []Vals) {
	stage.M.Lock()
	n := len(stage.Recycled)
	if n > 0 {
		rv = stage.Recycled[n-1]
		stage.Recycled[n-1] = nil
		stage.Recycled = stage.Recycled[0 : n-1]
	}
	stage.M.Unlock()

	return rv
}