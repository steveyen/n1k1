package n1k1

import (
	"math"

	"github.com/couchbase/n1k1/base"
)

func OpOrderByOffsetLimit(o *base.Op, lzYieldVals base.YieldVals,
	lzYieldStats base.YieldStats, lzYieldErr base.YieldErr, path, pathNext string) {
	projections := o.Params[0].([]interface{}) // ORDER BY expressions.

	// Then directions has same len as projections, ex: ["asc", "desc", "asc"].
	directions := o.Params[1].([]interface{})

	offset := 0

	limit := math.MaxInt64

	if len(o.Params) >= 3 {
		offset = o.Params[2].(int)

		if len(o.Params) >= 4 {
			limit = o.Params[3].(int)
		}
	}

	if LzScope {
		pathNextOOL := EmitPush(pathNext, "OOL") // !lz

		var lzProjectFunc base.ProjectFunc
		_ = lzProjectFunc

		lzProjectFunc =
			MakeProjectFunc(o.ParentA.Fields, nil, projections, pathNextOOL, "PF") // !lz

		var lzLessFunc base.LessFunc
		_ = lzLessFunc

		lzLessFunc =
			MakeLessFunc(nil, directions) // !lz

		var lzItems []base.Vals // Items collected to be sorted.

		lzYieldValsOrig := lzYieldVals

		lzYieldVals = func(lzVals base.Vals) {
			lzItem := make(base.Vals, 0, len(lzVals)) // TODO: Prealloc here?

			for _, lzVal := range lzVals { // Deep copy.
				lzItem = append(lzItem, append(base.Val(nil), lzVal...))
			}

			lzItems = append(lzItems, lzItem)

			// TODO: If no order-by, but OFFSET+LIMIT reached, early exit?
		}

		lzYieldErrOrig := lzYieldErr

		lzYieldErr = func(lzErrIn error) {
			if lzErrIn == nil { // If no error, yield our sorted items.
				nProjections := len(projections) // !lz
				if nProjections > 0 {            // !lz
					lzProjected := make([]base.Vals, 0, len(lzItems))
					lzInterfaces := make([][]interface{}, 0, len(lzItems))
					lzInterfacesAll := make([]interface{}, len(lzItems)*nProjections)

					for lzI, lzVals := range lzItems {
						var lzValsOut base.Vals

						lzValsOut = lzProjectFunc(lzVals, lzValsOut) // <== emitCaptured: pathNextOOL "PF"

						lzProjected = append(lzProjected, lzValsOut)

						lzInterfaces = append(lzInterfaces, lzInterfacesAll[lzI*nProjections:(lzI+1)*nProjections])
					}

					base.OrderByItems(lzItems, lzProjected, lzInterfaces, lzLessFunc)
				} // !lz

				lzI := offset
				lzN := 0

				for lzI < len(lzItems) && lzN < limit {
					lzVals := lzItems[lzI]

					lzYieldValsOrig(lzVals)

					lzI++
					lzN++
				}
			}

			lzYieldErrOrig(lzErrIn)
		}

		EmitPop(pathNext, "OOL") // !lz

		if LzScope {
			ExecOp(o.ParentA, lzYieldVals, lzYieldStats, lzYieldErr, path, pathNext) // !lz
		}
	}
}

func MakeLessFunc(types base.Types, directions []interface{}) (
	lzLessFunc base.LessFunc) {
	// TODO: One day use types to optimize.

	if len(directions) > 0 { // !lz
		lzValComparer := &base.ValComparer{}

		lzLessFunc = func(lzValsA, lzValsB base.Vals, lzIA, lzIB []interface{}) bool {
			var lzCmp int

			for idx := range directions { // !lz
				direction := directions[idx] // !lz

				lt, gt := true, false                               // !lz
				if s, ok := direction.(string); ok && s == "desc" { // !lz
					lt, gt = false, true // !lz
				} // !lz

				lzCmp = lzValComparer.Compare(lzValsA[idx], lzValsB[idx], &lzIA[idx], &lzIB[idx])
				if lzCmp < 0 {
					return lt
				}

				if lzCmp > 0 {
					return gt
				}
			} // !lz

			return false
		}
	} // !lz

	return lzLessFunc
}
