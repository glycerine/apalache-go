package api

import "github.com/glycerine/g3/pkg/util/symbol"

// Unlike Z3_mk_string_symbol's empty C string, a serialized $ || token is
// already an existing empty-string symbol and must not become symbol::null.
func replayFixedpointRuleSymbol(owner *replayContextOwner, value z3ReplayValue) (symbol.Symbol, error) {
	if value.nullSymbol {
		return symbol.Symbol{}, nil
	}
	if value.numericSymbol {
		return symbol.Numerical(uint32(value.u)), nil
	}
	return owner.results.manager.Symbols.Intern(value.s)
}
