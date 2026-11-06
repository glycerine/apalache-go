package params

// Source: src/smt/params/qi_params.cpp:43-64; src/util/params.cpp:928-930.

import

// AppendTo resolves the checked parameter input in the parameter layer before
// passing a native value to the numeric output. Borrowed inputs remain confined
// to this call; no parameter context is retained by the stream.
utext "github.com/glycerine/g3/pkg/util/text"

func (v ParamsStringView) AppendTo(out *utext.NumericTextStream) error {
	if e := out.CheckOpen(); e != nil {
		return e
	}
	if !v.PresentField {
		return out.AppendNullableCString(nil)
	}
	text, e := v.String()
	if e != nil {
		return e
	}
	return out.AppendNullableCString(&text)
}
