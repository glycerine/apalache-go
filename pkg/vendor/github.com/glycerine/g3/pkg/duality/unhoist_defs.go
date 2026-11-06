package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// InitializationDefinition owns both sides of an initialization equality.
type InitializationDefinition struct {
	Variable *Expr
	Value    *Expr
}

func (d *InitializationDefinition) Close() error {
	if d == nil {
		return nil
	}
	var result error
	if d.Variable != nil {
		if err := d.Variable.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			d.Variable = nil
		}
	}
	if d.Value != nil {
		if err := d.Value.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			d.Value = nil
		}
	}
	return result
}

// GetDefs extracts equalities whose left operand is an uninterpreted constant,
// descending only through conjunctions. Later equalities overwrite earlier
// definitions, including revisits of shared conjunctions. No equality
// orientation or inequality normalization is performed.
// The caller owns each returned definition, including its map-key expression.
// Source: duality_rpfp.cpp, GetDefsRec/GetDefs (UnhoistLoop consumer).
func GetDefs(term *Expr) (out map[ast.ASTHandle]*InitializationDefinition, result error) {
	if err := term.open(); err != nil {
		return nil, err
	}
	c := term.value.object.context
	m := c.manager
	root, err := term.Raw()
	if err != nil {
		return nil, err
	}
	out = make(map[ast.ASTHandle]*InitializationDefinition)
	defer func() {
		if result != nil {
			for _, d := range out {
				result = errors.Join(result, d.Close())
			}
			out = nil
		}
	}()
	pending := []ast.ASTHandle{root}
	for len(pending) > 0 {
		h := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		kind, err := m.GetKind(h)
		if err != nil {
			return out, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := m.GetFamilyID(h)
		if err != nil {
			return out, err
		}
		if family != m.BasicFamilyID() {
			continue
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return out, err
		}
		switch op {
		case ast.ASTOpAnd:
			n, err := m.GetNumArgs(h)
			if err != nil {
				return out, err
			}
			for i := n; i > 0; i-- {
				child, err := m.GetArg(h, i-1)
				if err != nil {
					return out, err
				}
				pending = append(pending, child)
			}
		case ast.ASTOpEq:
			lhs, err := m.GetArg(h, 0)
			if err != nil {
				return out, err
			}
			kind, err := m.GetKind(lhs)
			if err != nil {
				return out, err
			}
			if kind != ast.ASTApp {
				continue
			}
			n, err := m.GetNumArgs(lhs)
			if err != nil {
				return out, err
			}
			family, err := m.GetFamilyID(lhs)
			if err != nil {
				return out, err
			}
			if n != 0 || family != ast.NullFamilyID {
				continue
			}
			rhs, err := m.GetArg(h, 1)
			if err != nil {
				return out, err
			}
			if old := out[lhs]; old != nil {
				if err := old.Close(); err != nil {
					return out, err
				}
				delete(out, lhs)
			}
			variable, err := NewExpr(c, lhs)
			if err != nil {
				return out, err
			}
			definition := &InitializationDefinition{Variable: variable}
			out[lhs] = definition
			definition.Value, err = NewExpr(c, rhs)
			if err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
