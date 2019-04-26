package n1k1

import (
	"strings"

	"github.com/couchbase/n1k1/base"
)

func ExecOperator(o *base.Operator, lzYieldVals base.YieldVals,
	lzYieldStats base.YieldStats, lzYieldErr base.YieldErr,
	path, pathItem string) {
	pathNext := EmitPush(path, pathItem)

	defer EmitPop(path, pathItem)

	if o == nil {
		return
	}

	switch o.Kind {
	case "scan":
		Scan(o.Params, o.Fields, lzYieldVals, lzYieldStats, lzYieldErr) // !lz

	case "filter":
		if LzScope {
			pathNextF := EmitPush(pathNext, "F") // !lz

			var lzExprFunc base.ExprFunc

			lzExprFunc =
				MakeExprFunc(o.ParentA.Fields, nil, o.Params, pathNextF, "FF") // !lz

			lzYieldValsOrig := lzYieldVals

			_, _ = lzExprFunc, lzYieldValsOrig

			lzYieldVals = func(lzVals base.Vals) {
				var lzVal base.Val

				lzVal = lzExprFunc(lzVals) // <== emitCaptured: pathNextF "FF"

				if base.ValEqualTrue(lzVal) {
					lzYieldValsOrig(lzVals) // <== emitCaptured: path ""
				}
			}

			EmitPop(pathNext, "F") // !lz

			ExecOperator(o.ParentA, lzYieldVals, lzYieldStats, lzYieldErr, pathNextF, "") // !lz
		}

	case "project":
		if LzScope {
			pathNextP := EmitPush(pathNext, "P") // !lz

			var lzValsReuse base.Vals // <== varLift: lzValsReuse by path

			var lzProjectFunc base.ProjectFunc

			lzProjectFunc =
				MakeProjectFunc(o.ParentA.Fields, nil, o.Params, pathNextP, "PF") // !lz

			lzYieldValsOrig := lzYieldVals

			_, _ = lzProjectFunc, lzYieldValsOrig

			lzYieldVals = func(lzVals base.Vals) {
				lzValsOut := lzValsReuse[:0]

				lzValsOut = lzProjectFunc(lzVals, lzValsOut) // <== emitCaptured: pathNextP "PF"

				lzValsReuse = lzValsOut

				lzYieldValsOrig(lzValsOut)
			}

			EmitPop(pathNext, "P") // !lz

			ExecOperator(o.ParentA, lzYieldVals, lzYieldStats, lzYieldErr, pathNextP, "") // !lz
		}

	case "join-inner-nl":
		ExecJoinNestedLoop(o, lzYieldVals, lzYieldStats, lzYieldErr, path, pathNext) // !lz

	case "join-outerLeft-nl":
		ExecJoinNestedLoop(o, lzYieldVals, lzYieldStats, lzYieldErr, path, pathNext) // !lz
	}
}

func ExecJoinNestedLoop(o *base.Operator, lzYieldVals base.YieldVals,
	lzYieldStats base.YieldStats, lzYieldErr base.YieldErr,
	path, pathNext string) {
	joinKind := strings.Split(o.Kind, "-")[1] // Ex: "inner", "outerLeft".

	lenFieldsA := len(o.ParentA.Fields)
	lenFieldsB := len(o.ParentB.Fields)
	lenFieldsAB := lenFieldsA + lenFieldsB

	fieldsAB := make(base.Fields, 0, lenFieldsAB)
	fieldsAB = append(fieldsAB, o.ParentA.Fields...)
	fieldsAB = append(fieldsAB, o.ParentB.Fields...)

	if LzScope {
		var lzExprFunc base.ExprFunc

		lzExprFunc =
			MakeExprFunc(fieldsAB, nil, o.Params, pathNext, "JF") // !lz

		var lzHadInner bool

		_, _ = lzExprFunc, lzHadInner

		lzValsJoin := make(base.Vals, lenFieldsAB)

		lzYieldValsOrig := lzYieldVals

		lzYieldVals = func(lzValsA base.Vals) {
			lzValsJoin = lzValsJoin[:0]
			lzValsJoin = append(lzValsJoin, lzValsA...)

			if joinKind == "outerLeft" { // !lz
				lzHadInner = false
			} // !lz

			if LzScope {
				lzYieldVals := func(lzValsB base.Vals) {
					lzValsJoin = lzValsJoin[0:lenFieldsA]
					lzValsJoin = append(lzValsJoin, lzValsB...)

					if joinKind == "outerLeft" { // !lz
						lzHadInner = true
					} // !lz

					lzVals := lzValsJoin

					_ = lzVals

					var lzVal base.Val

					lzVal = lzExprFunc(lzVals) // <== emitCaptured: pathNext, "JF"
					if base.ValEqualTrue(lzVal) {
						lzYieldValsOrig(lzVals) // <== emitCaptured: path ""
					} else {
						if joinKind == "outerLeft" { // !lz
							lzValsJoin = lzValsJoin[0:lenFieldsA]
							for i := 0; i < lenFieldsB; i++ { // !lz
								lzValsJoin = append(lzValsJoin, base.ValMissing)
							} // !lz

							lzYieldValsOrig(lzValsJoin)
						} // !lz
					}
				}

				// Inner (right)...
				ExecOperator(o.ParentB, lzYieldVals, lzYieldStats, lzYieldErr, pathNext, "JNLI") // !lz

				// Case of outerLeft join when inner (right) was empty.
				if joinKind == "outerLeft" { // !lz
					if !lzHadInner {
						lzValsJoin = lzValsJoin[0:lenFieldsA]
						for i := 0; i < lenFieldsB; i++ { // !lz
							lzValsJoin = append(lzValsJoin, base.ValMissing)
						} // !lz

						lzYieldValsOrig(lzValsJoin)
					}
				} // !lz
			}
		}

		// Outer (left)...
		ExecOperator(o.ParentA, lzYieldVals, lzYieldStats, lzYieldErr, pathNext, "JNLO") // !lz
	}
}
