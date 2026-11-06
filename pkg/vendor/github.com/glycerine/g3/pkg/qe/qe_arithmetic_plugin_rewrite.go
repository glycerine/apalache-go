package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// errArithmeticBranchUnsupported reports an arithmetic branch whose bound
// resolution is not yet implemented by the concrete plugin.
var errArithmeticBranchUnsupported = errors.New("qe: arithmetic branch resolution is not implemented")

func (p *ArithmeticPlugin) replaceAtom(result ast.ASTRef, atom ast.ASTRef, replacement ast.ASTHandle) error {
	a, err := p.manager.RefGet(atom)
	if err != nil {
		return err
	}
	return p.replacer.ApplySubstitution(a, replacement, result)
}

func (p *ArithmeticPlugin) rewriteInfinity(entry *arithmeticPluginBounds, lower bool, result ast.ASTRef) error {
	truth, err := p.manager.MkTrue()
	if err != nil {
		return err
	}
	falsity, err := p.manager.MkFalse()
	if err != nil {
		return err
	}
	selected := ArithmeticUpperBound
	if lower {
		selected = ArithmeticLowerBound
	}
	opposite := ArithmeticLowerBound
	if lower {
		opposite = ArithmeticUpperBound
	}
	for _, strict := range []bool{false, true} {
		for _, b := range entry.processor.Bounds(strict, selected) {
			if err = p.replaceAtom(result, b.Atom, falsity); err != nil {
				return err
			}
			a, cause := p.manager.RefGet(b.Atom)
			if cause != nil {
				return cause
			}
			negated, cause := p.manager.MkNot(a)
			if cause != nil {
				return cause
			}
			if err = p.appendConstraint(true, negated); err != nil {
				return err
			}
		}
		for _, b := range entry.processor.Bounds(strict, opposite) {
			if err = p.replaceAtom(result, b.Atom, truth); err != nil {
				return err
			}
			a, cause := p.manager.RefGet(b.Atom)
			if cause != nil {
				return cause
			}
			if err = p.appendConstraint(true, a); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *ArithmeticPlugin) sameSide(selected, current *ArithmeticBound, lower, strict bool) (ast.ASTRef, error) {
	a := new(big.Rat).Set(selected.Coefficient)
	b := new(big.Rat).Set(current.Coefficient)
	if lower {
		a.Neg(a)
		b.Neg(b)
	}
	t, err := p.manager.RefGet(selected.Term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	s, err := p.manager.RefGet(current.Term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.util.MakeBound(a, t, b, s, strict)
}

func (p *ArithmeticPlugin) rewriteFinite(entry *arithmeticPluginBounds, target ast.ASTHandle, selection ArithmeticBranchSelection, result ast.ASTRef) error {
	selected := selection.Bound
	if selected == nil {
		return ast.ErrASTDomain
	}
	selectedTerm, err := p.manager.RefGet(selected.Term)
	if err != nil {
		return err
	}
	direction := selected.Direction
	for _, strict := range []bool{false, true} {
		for i, current := range entry.processor.Bounds(strict, direction) {
			isSelected := strict == selected.Strict && uint32(i) == selection.Index
			if isSelected {
				truth, cause := p.manager.MkTrue()
				if cause != nil {
					return cause
				}
				if cause = p.replaceAtom(result, current.Atom, truth); cause != nil {
					return cause
				}
				a, _ := p.manager.RefGet(current.Atom)
				if cause = p.appendConstraint(true, a); cause != nil {
					return cause
				}
				continue
			}
			resultStrict := strict == selected.Strict && uint32(i) < selection.Index
			if selection.Kind == ArithmeticEqualityBranch && strict {
				resultStrict = true
			}
			replacement, cause := p.sameSide(selected, current, selection.Lower, resultStrict)
			if cause != nil {
				return cause
			}
			h, cause := p.manager.RefGet(replacement)
			if cause == nil {
				cause = p.replaceAtom(result, current.Atom, h)
			}
			if cause == nil {
				a, _ := p.manager.RefGet(current.Atom)
				na, _ := p.manager.MkNot(a)
				imp, _ := p.manager.MkOrArray([]ast.ASTHandle{na, h})
				cause = p.appendConstraint(true, imp)
			}
			if closeErr := p.manager.DelRef(&replacement); cause == nil {
				cause = closeErr
			}
			if cause != nil {
				return cause
			}
		}
		opposite := ArithmeticLowerBound
		if direction == ArithmeticLowerBound {
			opposite = ArithmeticUpperBound
		}
		for _, current := range entry.processor.Bounds(strict, opposite) {
			s, err := p.manager.RefGet(current.Term)
			if err != nil {
				return err
			}
			strictResolve := strict || selected.Strict || selection.Kind == ArithmeticOpenBranch
			replacement, handled, cause := p.util.ResolveUnitOrReal(target, strictResolve, selected.Coefficient, selectedTerm, current.Coefficient, s)
			if cause != nil {
				return cause
			}
			if !handled {
				return errArithmeticBranchUnsupported
			}
			h, cause := p.manager.RefGet(replacement)
			if cause == nil {
				cause = p.replaceAtom(result, current.Atom, h)
			}
			if cause == nil {
				a, _ := p.manager.RefGet(current.Atom)
				na, _ := p.manager.MkNot(a)
				imp, _ := p.manager.MkOrArray([]ast.ASTHandle{na, h})
				cause = p.appendConstraint(true, imp)
			}
			if closeErr := p.manager.DelRef(&replacement); cause == nil {
				cause = closeErr
			}
			if cause != nil {
				return cause
			}
		}
	}
	return nil
}
