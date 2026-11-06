package opt

// Source: src/opt/maxsmt.cpp/model label value-copy semantics.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func cloneLabels(source []usymbol.Symbol) []usymbol.Symbol {
	return append([]usymbol.Symbol(nil), source...)
}
