package n1k1

func MakeExprFunc(fields Fields, types Types, expr []interface{},
	outTypes Types) (lazyExprFunc LazyExprFunc) {
	f := ExprCatalog[expr[0].(string)]
	lazyExprFunc =
		f(fields, types, expr[1:], outTypes) // <== inline-ok.
	return lazyExprFunc
}

// -----------------------------------------------------

type LazyExprFunc func(lazyVals LazyVals) LazyVal

type ExprCatalogFunc func(fields Fields, types Types, params []interface{},
	outTypes Types) (lazyExprFunc LazyExprFunc)

var ExprCatalog = map[string]ExprCatalogFunc{}

func init() {
	ExprCatalog["eq"] = ExprEq
	ExprCatalog["json"] = ExprJson
	ExprCatalog["field"] = ExprField
}

// -----------------------------------------------------

var JsonTypes = map[byte]string{ // TODO: Use byte array instead?
	'"': "string",
	'{': "object",
	'[': "array",
	'n': "null",
	't': "bool", // From "true".
	'f': "bool", // From "false".
	'-': "number",
	'0': "number",
	'1': "number",
	'2': "number",
	'3': "number",
	'4': "number",
	'5': "number",
	'6': "number",
	'7': "number",
	'8': "number",
	'9': "number",
}

func ExprJson(fields Fields, types Types, params []interface{},
	outTypes Types) (lazyExprFunc LazyExprFunc) {
	json := params[0].(string) // TODO: Use []byte one day.
	jsonType := JsonTypes[json[0]] // Might be "".

	SetLastType(outTypes, jsonType)

	lazyExprFunc = func(lazyVals LazyVals) (lazyVal LazyVal) {
		return LazyVal(json)
	}

	return lazyExprFunc
}

// -----------------------------------------------------

func ExprField(fields Fields, types Types, params []interface{},
	outTypes Types) (lazyExprFunc LazyExprFunc) {
	idx := fields.IndexOf(params[0].(string))
	if idx < 0 {
		SetLastType(outTypes, "")
	} else {
		SetLastType(outTypes, types[idx])
	}

	lazyExprFunc = func(lazyVals LazyVals) (lazyVal LazyVal) {
		if idx < 0 {
			lazyVal = LazyValMissing
		} else {
			lazyVal = lazyVals[idx]
		}

		return lazyVal
	}

	return lazyExprFunc
}

// -----------------------------------------------------

func ExprEq(fields Fields, types Types, params []interface{},
	outTypes Types) (lazyExprFunc LazyExprFunc) {
	exprA := params[0].([]interface{})
	lazyExprFunc =
		MakeExprFunc(fields, types, exprA, outTypes) // <== inline-ok.
	lazyA := lazyExprFunc
	TakeLastType(outTypes)

	exprB := params[1].([]interface{})
	lazyExprFunc =
		MakeExprFunc(fields, types, exprB, outTypes) // <== inline-ok.
	lazyB := lazyExprFunc
	TakeLastType(outTypes)

	SetLastType(outTypes, "bool")

	lazyExprFunc = func(lazyVals LazyVals) (lazyVal LazyVal) {
		lazyVal =
			lazyA(lazyVals) // <== inline-ok.
		lazyValA := lazyVal

		lazyVal =
			lazyB(lazyVals) // <== inline-ok.
		lazyValB := lazyVal

		if lazyValA == LazyValMissing || lazyValB == LazyValMissing {
			lazyVal = LazyValMissing
		} else if lazyValA == LazyValNull || lazyValB == LazyValNull {
			lazyVal = LazyValNull
		} else if lazyValA == lazyValB {
			lazyVal = LazyValTrue
		} else {
			lazyVal = LazyValFalse
		}

		return lazyVal
	}

	return lazyExprFunc
}
