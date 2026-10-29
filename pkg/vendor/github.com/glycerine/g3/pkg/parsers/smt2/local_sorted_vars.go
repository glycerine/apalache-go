package smt2

// Source: src/parsers/smt2/smt2parser.cpp:1253-1280; src/util/symbol_table.h:142-182.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var errSortedVarInputs = errors.New("smt2 parser: sorted variable names and sorts differ")
var errSortedVarBindings = errors.New("smt2 parser: too many sorted variable bindings")

// bindSortedVariables is parse_sorted_vars after the names and sorts have been
// parsed. Source assigns indices in reverse order while inserting names in
// declaration order, and leaves the new symbol-table scope open for its frame.
func (e *smt2LocalEnv) bindSortedVariables(m *ast.ASTManager, names []symbol.Symbol, sorts []ast.ASTHandle, numBindings uint32, exprs *localExprStack) (uint32, error) {
	if e == nil || exprs == nil || len(names) != len(sorts) {
		return numBindings, errSortedVarInputs
	}
	if uint64(len(names))+uint64(numBindings) > math.MaxUint32 {
		return numBindings, errSortedVarBindings
	}
	base := len(exprs.refs)
	e.beginScope()
	level := numBindings + uint32(len(names))
	for i, name := range names {
		v, err := m.MkVar(uint32(len(names)-1-i), sorts[i])
		if err == nil {
			var ref ast.ASTRef
			ref, err = m.NewRef(v)
			if err == nil {
				exprs.pushOwned(ref)
				e.insert(name, smt2Local{term: v, level: level})
				debug := diagnostic.BorrowProcessDebugContext()
				var enabled bool
				enabled, err = diagnostic.SourceAssertionsEnabled(debug)
				if err == nil && enabled && !e.contains(name) {
					err = diagnostic.DebugAssertionFailure(debug, "src/parsers/smt2/smt2parser.cpp", 1279, "m_env.contains(*sym_it)")
				}
			}
		}
		if err != nil {
			return numBindings, errors.Join(err, e.endScopeWithOwningCleanup(), exprs.shrink(m, base))
		}
	}
	return level, nil
}
