package n1k1

// The LazyYield memory ownership rule: the receiver func should copy
// any inputs that it wants to keep, because the provided slices might
// be reused by future invocations.
type LazyYield func(LazyVals)

type LazyYieldErr func(error)

type Operator struct {
	Kind   string        // Ex: "scan", "filter", "project", etc.
	Fields Fields        // Output fields of this operator.
	Params []interface{} // Params based on the kind.

	ParentA *Operator
	ParentB *Operator
}

func ExecOperator(o *Operator,
	lazyYield LazyYield, lazyYieldErr LazyYieldErr) {
	if o == nil {
		return
	}

	switch o.Kind {
	case "scan":
		Scan(o.Params, lazyYield, lazyYieldErr) // <== inline-ok.

	case "filter":
		var lazyExprFunc LazyExprFunc

		types := make(Types, len(o.ParentA.Fields)) // TODO.
		outTypes := Types{""} // TODO.

		lazyExprFunc =
			MakeExprFunc(o.ParentA.Fields, types, o.Params, outTypes) // <== inline-ok.

		lazyYieldOrig := lazyYield

		lazyYield = func(lazyVals LazyVals) {
			lazyAny := lazyExprFunc(lazyVals)
			lazyBool := false
			if lazyAny != nil {
				if b, ok := lazyAny.(bool); ok && b { // TODO.
					lazyBool = true
				}
			}
			if lazyBool {
				lazyYieldOrig(lazyVals)
			}
		}

		ExecOperator(o.ParentA, lazyYield, lazyYieldErr) // <== inlineOk

	case "project":
		// TODO.
		// if len(o.Types) <= 0 && len(o.Fields) > 0 {
		//    o.Types = make(Types, len(o.Fields))
		// }

		types := make(Types, len(o.ParentA.Fields)) // TODO.
		outTypes := Types{""} // TODO.

		var lazyProjectFunc LazyProjectFunc

		lazyProjectFunc =
			MakeProjectFunc(o.ParentA.Fields, types, o.Params, outTypes) // <== inlineOk

		var lazyVals LazyVals

		lazyYieldOrig := lazyYield

		lazyYield = func(lazyValsIn LazyVals) {
			lazyVals = lazyProjectFunc(lazyValsIn, lazyVals[:0])

			lazyYieldOrig(lazyVals)
		}

		ExecOperator(o.ParentA, lazyYield, lazyYieldErr) // <== inlineOk

	case "join-nl": // Nested loop join.
		var fieldsAB Fields

		fieldsAB = append(fieldsAB, o.ParentA.Fields...)
		fieldsAB = append(fieldsAB, o.ParentB.Fields...)

		types := make(Types, len(fieldsAB)) // TODO.
		outTypes := Types{""} // TODO.

		var lazyExprFunc LazyExprFunc

		lazyExprFunc =
			MakeExprFunc(fieldsAB, types, o.Params, outTypes) // <== inline-ok.

		var lazyVals LazyVals

		lazyYieldOrig := lazyYield

		lazyYield = func(lazyValsA LazyVals) {
			lazyVals = lazyVals[:0]
			lazyVals = append(lazyVals, lazyValsA...)

			lazyYield = func(lazyValsB LazyVals) {
				lazyVals = lazyVals[0:len(lazyValsA)]
				lazyVals = append(lazyVals, lazyValsB...)

				lazyAny := lazyExprFunc(lazyVals)
				lazyBool := false
				if lazyAny != nil {
					if b, ok := lazyAny.(bool); ok && b { // TODO.
						lazyBool = true
					}
				}
				if lazyBool {
					lazyYieldOrig(lazyVals)
				}
			}

			// Inner...
			ExecOperator(o.ParentB, lazyYield, lazyYieldErr) // <== inlineOk
		}

		// Outer...
		ExecOperator(o.ParentA, lazyYield, lazyYieldErr) // <== inlineOk
	}
}
