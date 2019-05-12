package n1k1

import (
	"github.com/couchbase/n1k1/base"
)

func init() {
	ExprCatalog["eq"] = ExprEQ
	ExprCatalog["lt"] = ExprLT
	ExprCatalog["le"] = ExprLE
	ExprCatalog["gt"] = ExprGT
	ExprCatalog["ge"] = ExprGE
}

// -----------------------------------------------------

func ExprEQ(lzVars *base.Vars, labels base.Labels,
	params []interface{}, path string) (lzExprFunc base.ExprFunc) {
	return ExprCmp(lzVars, labels, params, path, EQCmps, base.ValTrue)
}

func ExprLT(lzVars *base.Vars, labels base.Labels,
	params []interface{}, path string) (lzExprFunc base.ExprFunc) {
	return ExprCmp(lzVars, labels, params, path, LTCmps, base.ValFalse)
}

func ExprLE(lzVars *base.Vars, labels base.Labels,
	params []interface{}, path string) (lzExprFunc base.ExprFunc) {
	return ExprCmp(lzVars, labels, params, path, LTCmps, base.ValTrue)
}

func ExprGT(lzVars *base.Vars, labels base.Labels,
	params []interface{}, path string) (lzExprFunc base.ExprFunc) {
	return ExprLT(lzVars, labels, []interface{}{params[1], params[0]}, path)
}

func ExprGE(lzVars *base.Vars, labels base.Labels,
	params []interface{}, path string) (lzExprFunc base.ExprFunc) {
	return ExprLE(lzVars, labels, []interface{}{params[1], params[0]}, path)
}

// -----------------------------------------------------

var EQCmps = []base.Val{base.ValFalse, base.ValFalse}

var LTCmps = []base.Val{base.ValTrue, base.ValFalse}

// -----------------------------------------------------

func ExprCmp(lzVars *base.Vars, labels base.Labels, params []interface{},
	path string, cmps []base.Val, cmpEQ base.Val) (lzExprFunc base.ExprFunc) {
	for parami, param := range params {
		expr := param.([]interface{})
		if expr[0].(string) == "json" { // Optimize when param is static JSON.
			return ExprCmpStatic(lzVars, labels, params, path, cmps, cmpEQ, parami)
		}
	}

	return ExprCmpDynamic(lzVars, labels, params, path, cmps, cmpEQ)
}

// -----------------------------------------------------

// ExprCmpStatic optimizes when params[parami] is static.
func ExprCmpStatic(lzVars *base.Vars, labels base.Labels,
	params []interface{}, path string, cmps []base.Val, cmpEQ base.Val,
	parami int) (lzExprFunc base.ExprFunc) {
	json := params[parami].([]interface{})[1].(string)

	staticVal, staticType := base.Parse([]byte(json))

	staticTypeHasValue := base.ParseTypeHasValue(staticType)

	var staticF64 float64 // Optimize further when static is number.
	var staticF64Ok bool

	if base.ParseTypeToValType[staticType] == base.ValTypeNumber {
		var err error

		staticF64, err = base.ParseFloat64(staticVal)
		if err == nil {
			staticF64Ok = true
		}
	}

	exprX := params[(parami+1)%2].([]interface{})

	cmpLT, cmpGT := cmps[0], cmps[1] // Ex: static < expr.
	if parami == 1 {
		cmpLT, cmpGT = cmps[1], cmps[0] // Ex: expr < static.
	}

	if LzScope {
		var lzCmpLT base.Val = cmpLT // <== varLift: lzCmpLT by path
		var lzCmpEQ base.Val = cmpEQ // <== varLift: lzCmpEQ by path
		var lzCmpGT base.Val = cmpGT // <== varLift: lzCmpGT by path

		lzExprFunc =
			MakeExprFunc(lzVars, labels, exprX, path, "X") // !lz
		lzX := lzExprFunc

		var lzValStatic base.Val = base.Val(staticVal) // <== varLift: lzValStatic by path

		lzExprFunc = func(lzVals base.Vals, lzYieldErr base.YieldErr) (lzVal base.Val) {
			if LzScope {
				if !staticTypeHasValue { // !lz
					lzVal = lzValStatic
				} else { // !lz
					lzVal = lzX(lzVals, lzYieldErr) // <== emitCaptured: path "X"

					lzValX, lzTypeX := base.Parse(lzVal)
					if base.ParseTypeHasValue(lzTypeX) {
						lzCmpNeeded := true

						if staticF64Ok { // !lz
							if base.ParseTypeToValType[lzTypeX] == base.ValTypeNumber {
								lzF64, lzErr := base.ParseFloat64(lzValX)
								if lzErr == nil {
									lzCmpNeeded = false

									if staticF64 < lzF64 {
										lzVal = lzCmpLT
									} else if staticF64 == lzF64 {
										lzVal = lzCmpEQ
									} else {
										lzVal = lzCmpGT
									}
								}
							}
						} // !lz

						if lzCmpNeeded {
							lzCmp := lzVars.Ctx.ValComparer.CompareWithType(lzValStatic, lzValX, staticType, lzTypeX, 0)
							if lzCmp < 0 {
								lzVal = lzCmpLT
							} else if lzCmp == 0 {
								lzVal = lzCmpEQ
							} else {
								lzVal = lzCmpGT
							}
						}
					}
				} // !lz
			}

			return lzVal
		}
	}

	return lzExprFunc
}

// -----------------------------------------------------

// Expressions A & B need to be runtime evaluated.
func ExprCmpDynamic(lzVars *base.Vars, labels base.Labels,
	params []interface{}, path string, cmps []base.Val, cmpEQ base.Val) (
	lzExprFunc base.ExprFunc) {
	cmpLT, cmpGT := cmps[0], cmps[1]

	var lzCmpLT base.Val = cmpLT // <== varLift: lzCmpLT by path
	var lzCmpEQ base.Val = cmpEQ // <== varLift: lzCmpEQ by path
	var lzCmpGT base.Val = cmpGT // <== varLift: lzCmpGT by path

	biExprFunc := func(lzA, lzB base.ExprFunc, lzVals base.Vals, lzYieldErr base.YieldErr) (lzVal base.Val) { // !lz
		if LzScope {
			lzVal = lzA(lzVals, lzYieldErr) // <== emitCaptured: path "A"

			lzValA, lzTypeA := base.Parse(lzVal)
			if base.ParseTypeHasValue(lzTypeA) {
				lzVal = lzB(lzVals, lzYieldErr) // <== emitCaptured: path "B"

				lzValB, lzTypeB := base.Parse(lzVal)
				if base.ParseTypeHasValue(lzTypeB) {
					lzCmp := lzVars.Ctx.ValComparer.CompareWithType(lzValA, lzValB, lzTypeA, lzTypeB, 0)
					if lzCmp < 0 {
						lzVal = lzCmpLT
					} else if lzCmp == 0 {
						lzVal = lzCmpEQ
					} else {
						lzVal = lzCmpGT
					}
				}
			}
		}

		return lzVal
	} // !lz

	lzExprFunc =
		MakeBiExprFunc(lzVars, labels, params, path, biExprFunc) // !lz

	return lzExprFunc
}
