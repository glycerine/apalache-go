package qe

// Source: src/qe/qe_dl_plugin.cpp, assign and subst.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *DatalogPlugin) branch(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) (arithmeticPluginKey, *datalogQECache, uint32, bool, error) {
	if value == nil {
		return arithmeticPluginKey{}, nil, 0, false, ast.ErrASTHandle
	}
	key, e := p.key(variable, formula)
	if e != nil {
		return key, nil, 0, false, e
	}
	entry := p.entries[key]
	if entry == nil {
		return key, nil, 0, false, ast.ErrASTDomain
	}
	service := p.manager.NumeralService()
	unsigned, e := service.IsUnsigned(value)
	if e != nil {
		return key, nil, 0, false, e
	}
	if !unsigned {
		return key, nil, 0, false, ast.ErrASTDomain
	}
	v, e := service.Unsigned(value)
	if e != nil {
		return key, nil, 0, false, e
	}
	sort, e := p.manager.GetSort(key.variable)
	if e != nil {
		return key, nil, 0, false, e
	}
	size, e := p.manager.GetSortSize(sort)
	if e != nil {
		return key, nil, 0, false, e
	}
	if !size.IsFinite() {
		return key, nil, 0, false, ast.ErrASTDomain
	}
	domain, e := size.Size()
	if e != nil {
		return key, nil, 0, false, e
	}
	count, small := datalogQEBranchCount(domain, entry.atoms)
	if uint64(v) >= count {
		return key, nil, 0, false, ast.ErrASTDomain
	}
	return key, entry, v, small, nil
}

func (p *DatalogPlugin) addConstraint(h ast.ASTHandle) error {
	ref, e := p.manager.NewRef(h)
	if e != nil {
		return e
	}
	p.constraints = append(p.constraints, ArithmeticConstraint{UseVariable: true, Formula: ref})
	return nil
}

// Assign publishes the source branch constraints in atom order.
func (p *DatalogPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) (err error) {
	key, entry, v, small, err := p.branch(variable, formula, value)
	if err != nil {
		return err
	}
	if small {
		sort, e := p.manager.GetSort(key.variable)
		if e != nil {
			return e
		}
		numeral, e := p.manager.MkDatalogNumeral(uint64(v), sort)
		if e != nil {
			return e
		}
		ref, e := p.manager.NewRef(numeral)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, p.manager.DelRef(&ref)) }()
		eq, e := p.manager.MkEq(key.variable, numeral)
		if e != nil {
			return e
		}
		return p.addConstraint(eq)
	}
	if int(v) < len(entry.atoms.positive) {
		return p.addConstraint(entry.atoms.positive[v].atom)
	}
	for _, atoms := range [][]datalogQEAtom{entry.atoms.positive, entry.atoms.negative} {
		for _, a := range atoms {
			neg, e := p.manager.MkNot(a.atom)
			if e != nil {
				return e
			}
			if e = p.addConstraint(neg); e != nil {
				return e
			}
		}
	}
	return nil
}

// TakeConstraints transfers the generated expression owners to the caller.
func (p *DatalogPlugin) TakeConstraints() ([]ArithmeticConstraint, error) {
	if e := p.open(); e != nil {
		return nil, e
	}
	out := p.constraints
	p.constraints = nil
	return out, nil
}

// Substitute applies the source cofactor and clears the optional definition.
func (p *DatalogPlugin) Substitute(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) (err error) {
	if e := p.open(); e != nil {
		return e
	}
	original, e := p.manager.RefGet(formula)
	if e != nil {
		return e
	}
	key, entry, v, small, e := p.branch(variable, original, value)
	if e != nil {
		return e
	}
	rw, e := rewriter.NewExprSafeReplace(p.manager)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	if small {
		sort, e := p.manager.GetSort(key.variable)
		if e != nil {
			return e
		}
		replacement, e := p.manager.MkDatalogNumeral(uint64(v), sort)
		if e != nil {
			return e
		}
		if e = rw.ApplySubstitution(key.variable, replacement, formula); e != nil {
			return e
		}
	} else if int(v) < len(entry.atoms.positive) {
		if e = rw.ApplySubstitution(key.variable, entry.atoms.positive[v].value, formula); e != nil {
			return e
		}
	} else {
		// Source substitutes false for positive equality atoms and true for
		// negative equality atoms in the other-value branch.
		for _, a := range entry.atoms.positive {
			replacement, e := p.manager.MkBoolVal(false)
			if e != nil {
				return e
			}
			if e = rw.ApplySubstitution(a.atom, replacement, formula); e != nil {
				return e
			}
		}
		for _, a := range entry.atoms.negative {
			// Negative literals store their underlying equality atom. In the
			// "other value" branch that equality is false, so its negation
			// remains true in the projected formula.
			replacement, e := p.manager.MkBoolVal(false)
			if e != nil {
				return e
			}
			if e = rw.ApplySubstitution(a.atom, replacement, formula); e != nil {
				return e
			}
		}
	}
	if definition != nil {
		return p.manager.SetRef(*definition, ast.ASTHandle{})
	}
	return nil
}
