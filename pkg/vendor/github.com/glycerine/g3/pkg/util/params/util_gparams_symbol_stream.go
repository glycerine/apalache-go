package params

// Source: src/util/symbol.h:97-110; src/smt/params/smt_params.cpp:138.

import (
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (g *GlobalParamsService) AppendSymbolTo(out *utext.NumericTextStream, s usymbol.Symbol) error {
	enabled, e := g.NamespaceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && s.IsMarked() {
		if e = g.NamespaceAssertionFailure("src/util/symbol.h", 98, "!s.is_marked()"); e != nil {
			return e
		}
	}
	if s.Tag == 0 {
		if s.Value == 0 {
			return out.AppendCString("null")
		}
		// Resolve the checked immutable spelling in this parameter-service layer,
		// including its lifetime read after an earlier output failure.
		if !g.Symbols.Owns(s) {
			return usymbol.ErrOwner
		}
		return (ParamsStringView{symbols: g.Symbols, symbol: s, Mode: 3, PresentField: true}).AppendTo(out)
	}
	if e = out.AppendCString("k!"); e != nil {
		return e
	}
	// UNBOXINT in tptr.h casts the unboxed word to int before ostream
	// insertion, so numeric flags and the signed 32-bit boundary apply.
	return out.AppendInt32(int32(s.Value))
}
