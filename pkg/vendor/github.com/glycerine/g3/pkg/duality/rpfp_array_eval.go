package duality

// Source: duality_rpfp.cpp:1527-1576.
import (
	"github.com/glycerine/g3/pkg/ast"
)

// arrayModelValue borrows handles from the input expression and current model.
// It is local to one comparison, so neither owner can expire during its use.
type arrayModelValue struct {
	defined  bool
	entries  map[ast.ASTHandle]ast.ASTHandle
	fallback ast.ASTHandle
}

func (g *RPFP) evalArrayTerm(term ast.ASTHandle) (arrayModelValue, error) {
	m := g.context.manager
	var out arrayModelValue
	kind, err := m.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return out, err
	}
	family, err := m.GetFamilyID(term)
	if err != nil {
		return out, err
	}
	array, err := m.ArrayFamilyID()
	if err != nil || family != array {
		return out, err
	}
	op, err := m.GetDeclKind(term)
	if err != nil {
		return out, err
	}
	switch op {
	case ast.ASTArrayOpAsArray:
		decl, err := m.AsArrayFunction(term)
		if err != nil {
			return out, err
		}
		function, err := g.dualModel.FunctionInterpretation(decl)
		if err != nil {
			return out, err
		}
		if function == nil {
			return out, ErrWrapperOwner
		}
		out.entries = make(map[ast.ASTHandle]ast.ASTHandle)
		for i := uint32(0); i < function.NumEntries(); i++ {
			entry, err := function.Entry(i)
			if err != nil {
				return out, err
			}
			index, err := entry.Argument(0)
			if err != nil {
				return out, err
			}
			value, err := entry.Result()
			if err != nil {
				return out, err
			}
			out.entries[index] = value
		}
		out.fallback, err = function.Else()
		if err != nil {
			return out, err
		}
		out.defined = true
	case ast.ASTArrayOpStore:
		base, err := m.GetArg(term, 0)
		if err != nil {
			return out, err
		}
		out, err = g.evalArrayTerm(base)
		if err != nil || !out.defined {
			return out, err
		}
		index, err := m.GetArg(term, 1)
		if err != nil {
			return out, err
		}
		value, err := m.GetArg(term, 2)
		if err != nil {
			return out, err
		}
		for _, h := range []ast.ASTHandle{index, value} {
			numeral, err := g.arrayStoreNumeral(h)
			if err != nil {
				return out, err
			}
			if !numeral {
				out.defined = false
				return out, nil
			}
		}
		if value == out.fallback {
			delete(out.entries, index)
		} else {
			out.entries[index] = value
		}
	}
	return out, nil
}

// The source wrapper's is_numeral recognizes arithmetic unique values only.
func (g *RPFP) arrayStoreNumeral(term ast.ASTHandle) (bool, error) {
	return sourceArithmeticNumeral(g.context.manager, term)
}

func sourceArithmeticNumeral(m *ast.ASTManager, term ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(term)
	if err != nil {
		return false, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil || family != arithmetic {
		return false, err
	}
	return m.IsUniqueValue(term)
}

// EvalArrayEquality compares source finite function tables and arithmetic
// stores. Unsupported representations return an owning copy of the input.
func (g *RPFP) EvalArrayEquality(formula *Expr) (*Expr, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if g.dualModel == nil {
		return nil, ErrWrapperOwner
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	h, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	m := g.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTApp {
		return nil, ast.ErrASTDomain
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return nil, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return nil, err
	}
	if family != m.BasicFamilyID() || op != ast.ASTOpEq {
		return nil, ast.ErrASTDomain
	}
	left, err := m.GetArg(h, 0)
	if err != nil {
		return nil, err
	}
	right, err := m.GetArg(h, 1)
	if err != nil {
		return nil, err
	}
	lhs, err := g.evalArrayTerm(left)
	if err != nil {
		return nil, err
	}
	rhs, err := g.evalArrayTerm(right)
	if err != nil {
		return nil, err
	}
	if !lhs.defined || !rhs.defined {
		return formula.Clone()
	}
	equal := lhs.fallback == rhs.fallback && len(lhs.entries) == len(rhs.entries)
	if equal {
		for index, value := range lhs.entries {
			other, ok := rhs.entries[index]
			if !ok || value != other {
				equal = false
				break
			}
		}
	}
	return g.context.BoolVal(equal)
}
