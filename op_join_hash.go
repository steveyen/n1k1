package n1k1

import (
	"encoding/binary" // <== genCompiler:hide

	"strings"

	"github.com/couchbase/rhmap/store" // <== genCompiler:hide

	"github.com/couchbase/n1k1/base"
)

// OpJoinHash implements...
//  feature:            info tracked in probe map values:     yieldsUnprobed:
//   joinHash-inner      [                         leftVals ]  f
//   joinHash-outerLeft  [ tracksProbing           leftVals ]  t
//   intersect-all       [               leftCount          ]  f
//   intersect-distinct  [ tracksProbing                    ]  f
//   except-all          [ tracksProbing leftCount          ]  t
//   except-distinct     [ tracksProbing                    ]  t
func OpJoinHash(o *base.Op, lzVars *base.Vars, lzYieldVals base.YieldVals,
	lzYieldErr base.YieldErr, path, pathNext string) {
	kindParts := strings.Split(o.Kind, "-")

	opIntersect := kindParts[0] == "intersect"

	var exprLeft, exprRight []interface{}

	var canonical, tracksProbing, leftCount, leftVals, yieldsUnprobed bool

	if kindParts[0] == "joinHash" {
		exprLeft = o.Params[0].([]interface{})
		exprRight = o.Params[1].([]interface{})

		canonical = false

		if kindParts[1] == "outerLeft" {
			tracksProbing, yieldsUnprobed = true, true
		}

		leftVals = true
	} else {
		exprLeft = []interface{}{"valsCanonical"}
		exprRight = []interface{}{"valsCanonical"}

		canonical = true

		if o.Kind != "intersect-all" {
			tracksProbing = true
		}

		if kindParts[1] == "all" {
			leftCount = true
		}

		yieldsUnprobed = kindParts[0] == "except"
	}

	// ---------------------------------------------------------------

	if LzScope {
		var lzBytes8, lzOne8, lzZero8 [8]byte

		binary.LittleEndian.PutUint64(lzOne8[:], uint64(1))

		var lzLeftBytes []byte

		lzLeftBytes = append(lzLeftBytes, lzZero8[:]...) // Chain ends at offset 0.
		lzLeftBytes = append(lzLeftBytes, lzZero8[:]...) // Chain ends at size 0.

		// TODO: Configurable initial size for RHStore, and reusable RHStore.
		// TODO: Reuse backing bytes for lzMap.
		// TODO: Allow spill out to disk.
		lzMap := store.NewRHStore(97)

		var lzVal, lzValOut base.Val

		var lzValsOut base.Vals

		var lzProbeValNew []byte

		var lzErr error

		_, _ = lzBytes8, lzValOut

		exprLeftFunc :=
			MakeExprFunc(lzVars, o.Children[0].Labels, exprLeft, pathNext, "JHL") // !lz

		exprRightFunc :=
			MakeExprFunc(lzVars, o.Children[1].Labels, exprRight, pathNext, "JHR") // !lz

		EmitPush(pathNext, "JHF") // !lz

		lzYieldValsOrig := lzYieldVals

		// Callback for left side, to fill the probe map.
		lzYieldVals = func(lzVals base.Vals) {
			lzVal = exprLeftFunc(lzVals, lzYieldErr) // <== emitCaptured: pathNext "JHL"

			lzProbeKey := lzVal
			if !canonical { // !lz
				lzProbeKey, lzErr = lzVars.Ctx.ValComparer.CanonicalJSON(lzProbeKey, lzValOut[:0])

				lzValOut = lzProbeKey[:0]
			} // !lz

			if lzErr == nil && base.ValHasValue(lzProbeKey) {
				lzProbeVal, lzProbeKeyFound := lzMap.Get([]byte(lzProbeKey))
				if !lzProbeKeyFound {
					// Set first-time probe value into map.
					lzProbeValNew = lzProbeValNew[:0]

					if tracksProbing { // !lz
						lzProbeValNew = append(lzProbeValNew, byte(0))
					} // !lz

					if leftCount { // !lz
						lzProbeValNew = append(lzProbeValNew, lzOne8[:]...)
					} // !lz

					if leftVals { // !lz
						lzLeftBytesLen := len(lzLeftBytes)

						// End or tail of the chain has offset/size of 0.
						lzLeftBytes = append(lzLeftBytes, lzZero8[:]...)
						lzLeftBytes = append(lzLeftBytes, lzZero8[:]...)

						lzLeftBytes = base.ValsJoin(lzVals, lzLeftBytes)

						binary.LittleEndian.PutUint64(lzBytes8[:], uint64(lzLeftBytesLen))
						lzProbeValNew = append(lzProbeValNew, lzBytes8[:]...) // The offset into lzLeftBytes.
						binary.LittleEndian.PutUint64(lzBytes8[:], uint64(len(lzLeftBytes)-lzLeftBytesLen))
						lzProbeValNew = append(lzProbeValNew, lzBytes8[:]...) // The size.
					} // !lz

					lzMap.Set(store.Key(lzProbeKey), lzProbeValNew)
				} else {
					// Not the first time that we're seeing this probe
					// key, so increment its leftCount, append to its
					// leftVals chain, etc.
					lzProbeValNew = lzProbeValNew[:0]

					lzProbeValOld := lzProbeVal

					if tracksProbing { // !lz
						lzProbeValNew = append(lzProbeValNew, byte(0))
						lzProbeValOld = lzProbeValOld[1:]
					} // !lz

					if leftCount { // !lz
						lzLeftCount := binary.LittleEndian.Uint64(lzProbeValOld[:8])
						binary.LittleEndian.PutUint64(lzBytes8[:], lzLeftCount+1)
						lzProbeValNew = append(lzProbeValNew, lzBytes8[:]...)
						lzProbeValOld = lzProbeValOld[8:]
					} // !lz

					if leftVals { // !lz
						lzLeftBytesLen := len(lzLeftBytes)

						// Copy previous 'offset/size into lzLeftBytes' into lzLeftBytes.
						lzLeftBytes = append(lzLeftBytes, lzProbeValOld...)

						lzLeftBytes = base.ValsJoin(lzVals, lzLeftBytes)

						binary.LittleEndian.PutUint64(lzBytes8[:], uint64(lzLeftBytesLen))
						lzProbeValNew = append(lzProbeValNew, lzBytes8[:]...) // The offset into lzLeftBytes.
						binary.LittleEndian.PutUint64(lzBytes8[:], uint64(len(lzLeftBytes)-lzLeftBytesLen))
						lzProbeValNew = append(lzProbeValNew, lzBytes8[:]...) // The size.
					} // !lz

					copy(lzProbeVal, lzProbeValNew)
				}
			}
		}

		var lzErrLeft error

		lzYieldErrOrig := lzYieldErr

		lzYieldErr = func(lzErrIn error) {
			if lzErrIn != nil {
				lzErrLeft = lzErrIn

				lzYieldErrOrig(lzErrIn)
			}
		}

		EmitPop(pathNext, "JHF") // !lz

		if LzScope {
			// Run the left side to fill the probe map.
			ExecOp(o.Children[0], lzVars, lzYieldVals, lzYieldErr, pathNext, "JHL") // !lz
		}

		// -----------------------------------------------------------

		if lzErrLeft == nil {
			EmitPush(pathNext, "JHP") // !lz

			// Callback for right side, to probe the probe map.
			lzYieldVals = func(lzVals base.Vals) {
				lzVal = exprRightFunc(lzVals, lzYieldErr) // <== emitCaptured: pathNext "JHR"

				lzProbeKey := lzVal
				if !canonical { // !lz
					lzProbeKey, lzErr = lzVars.Ctx.ValComparer.CanonicalJSON(lzProbeKey, lzValOut[:0])

					lzValOut = lzProbeKey[:0]
				} // !lz

				if lzErr == nil && base.ValHasValue(lzProbeKey) {
					lzProbeVal, lzProbeKeyFound := lzMap.Get([]byte(lzProbeKey))
					if lzProbeKeyFound {
						if tracksProbing { // !lz
							if lzProbeVal[0] == byte(0) {
								lzProbeVal[0] = byte(1) // Mark as probed.

								if opIntersect { // !lz
									// Ex: intersect-distinct.
									lzValsOut = base.ValsSplit(lzProbeKey, lzValsOut[:0])

									lzYieldValsOrig(lzValsOut)
								} // !lz
							}

							lzProbeVal = lzProbeVal[1:]
						} // !lz

						if leftCount { // !lz
							if opIntersect { // !lz
								// Ex: intersect-all.
								lzValsOut = base.ValsSplit(lzProbeKey, lzValsOut[:0])

								lzLeftCount := binary.LittleEndian.Uint64(lzProbeVal[:8])

								for lzI := uint64(0); lzI < lzLeftCount; lzI++ {
									lzYieldValsOrig(lzValsOut)
								}
							} // !lz

							lzProbeVal = lzProbeVal[8:]
						} // !lz

						if leftVals { // !lz
							// Ex: joinHash-inner, joinHash-outerLeft.
							lzValsOut = base.YieldChainedVals(lzYieldValsOrig, lzVals, lzLeftBytes, lzProbeVal, lzValsOut)
						} // !lz
					}
				}
			}

			lzYieldErr = func(lzErrIn error) {
				if lzErrIn == nil {
					// If no error, yield unprobed items if needed (ex: outerLeft, except).

					if tracksProbing && yieldsUnprobed { // !lz
						rightLabelsLen := len(o.Children[1].Labels) // !lz
						_ = rightLabelsLen                          // !lz

						lzRightSuffix := make(base.Vals, rightLabelsLen)
						_ = lzRightSuffix

						lzMapVisitor := func(lzProbeKey store.Key, lzProbeVal store.Val) bool {
							if lzProbeVal[0] == byte(0) { // Unprobed.
								lzProbeVal = lzProbeVal[1:]

								if leftCount { // !lz
									// Ex: except-all.
									lzLeftCount := binary.LittleEndian.Uint64(lzProbeVal[:8])

									lzValsOut = base.ValsSplit(lzProbeKey, lzValsOut[:0])

									for lzI := uint64(0); lzI < lzLeftCount; lzI++ {
										lzYieldValsOrig(lzValsOut)
									}

									lzProbeVal = lzProbeVal[8:]
								} // !lz

								if leftVals { // !lz
									// Ex: joinHash-outerLeft.
									lzValsOut = base.YieldChainedVals(lzYieldValsOrig, lzRightSuffix, lzLeftBytes, lzProbeVal, lzValsOut)
								} // !lz

								if !leftCount && !leftVals { // !lz
									// Ex: except-distinct.
									lzValsOut = base.ValsSplit(lzProbeKey, lzValsOut[:0])

									lzYieldValsOrig(lzValsOut)
								} // !lz
							}

							return true
						}

						lzMap.Visit(lzMapVisitor)
					} // !lz
				}

				lzYieldErrOrig(lzErrIn)
			}

			EmitPop(pathNext, "JHP") // !lz

			if LzScope {
				// Run the right side to probe the probe map.
				ExecOp(o.Children[1], lzVars, lzYieldVals, lzYieldErr, pathNext, "JHR") // !lz
			}
		}
	}
}
