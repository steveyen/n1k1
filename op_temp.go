package n1k1

import (
	"github.com/couchbase/rhmap/heap" // <== genCompiler:hide

	"github.com/couchbase/n1k1/base"
)

// OpTempCapture runs the child op, and captures any yielded vals
// into the vars as a named temp entry.
func OpTempCapture(o *base.Op, lzVars *base.Vars, lzYieldVals base.YieldVals,
	lzYieldErr base.YieldErr, path, pathNext string) {
	lzHeap, lzErr := lzVars.Ctx.AllocHeap()
	if lzErr != nil {
		lzYieldErr(lzErr)
	} else {
		var lzBytes []byte

		lzYieldVals = func(lzVals base.Vals) {
			lzErr = lzHeap.PushBytes(base.ValsEncode(lzVals, lzBytes[:0]))
			if lzErr != nil {
				lzYieldErr(lzErr)
			}
		}

		ExecOp(o.Children[0], lzVars, lzYieldVals, lzYieldErr, pathNext, "TC") // !lz

		if lzErr == nil {
			lzVars.TempSet(o.Params[0].(string), lzHeap)
		}
	}
}

// -----------------------------------------------------

// OpTempYield yields vals previously captured by OpTempCapture.
func OpTempYield(o *base.Op, lzVars *base.Vars, lzYieldVals base.YieldVals,
	lzYieldErr base.YieldErr, path, pathNext string) {
	var lzErr error

	lzResource, lzOk := lzVars.TempGet(o.Params[0].(string))
	if lzOk && lzResource != nil {
		lzHeap, lzOk := lzResource.(*heap.Heap)
		if lzOk {
			var lzBytes []byte
			var lzVals base.Vals

			for lzI := 0; lzI < lzHeap.Len() && lzErr == nil; lzI++ {
				lzBytes, lzErr = lzHeap.Get(lzI)
				if lzErr != nil {
					lzYieldErr(lzErr)
				}

				lzYieldVals(base.ValsDecode(lzBytes, lzVals[:0]))
			}
		}
	}

	lzYieldErr(lzErr)
}
