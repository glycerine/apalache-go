package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:333-438.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Engine) MkUnsignedMulNoOverflow(a, b []ast.ASTHandle, out ast.ASTRef) (result error) {
	if len(a) == 0 || len(a) != len(b) {
		return ast.ErrASTDomain
	}
	m := e.config.manager
	falsehood, err := m.MkFalse()
	if err != nil {
		return err
	}
	extendedA := append(append(make([]ast.ASTHandle, 0, len(a)+1), a...), falsehood)
	extendedB := append(append(make([]ast.ASTHandle, 0, len(b)+1), b...), falsehood)
	product, err := e.MkMultiplier(extendedA, extendedB)
	if err != nil {
		return err
	}
	refs := make([]ast.ASTRef, 4)
	defer func() { result = errors.Join(result, releaseRefs(m, product), releaseRefs(m, refs)) }()
	for i := range refs {
		refs[i], err = m.NewRef(falsehood)
		if err != nil {
			return err
		}
	}
	accumulated, violation, term, overflow := refs[0], refs[1], refs[2], refs[3]
	for i := 1; i < len(a); i++ {
		current, err := m.RefGet(accumulated)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpOr, accumulated, current, a[len(a)-i]); err != nil {
			return err
		}
		current, err = m.RefGet(accumulated)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpAnd, term, current, b[i]); err != nil {
			return err
		}
		termHandle, err := m.RefGet(term)
		if err != nil {
			return err
		}
		current, err = m.RefGet(violation)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpOr, violation, termHandle, current); err != nil {
			return err
		}
	}
	productBits, err := e.handles(product)
	if err != nil {
		return err
	}
	violationHandle, err := m.RefGet(violation)
	if err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpOr, overflow, productBits[len(a)], violationHandle); err != nil {
		return err
	}
	overflowHandle, err := m.RefGet(overflow)
	if err != nil {
		return err
	}
	return e.config.simplify(ast.ASTOpNot, out, overflowHandle)
}

func (e *Engine) signedMulBound(a, b []ast.ASTHandle, positiveOverflow bool, out ast.ASTRef) (result error) {
	if len(a) == 0 || len(a) != len(b) {
		return ast.ErrASTDomain
	}
	m := e.config.manager
	falsehood, err := m.MkFalse()
	if err != nil {
		return err
	}
	extendedA := append(append(make([]ast.ASTHandle, 0, len(a)+1), a...), a[len(a)-1])
	extendedB := append(append(make([]ast.ASTHandle, 0, len(b)+1), b...), b[len(b)-1])
	product, err := e.MkMultiplier(extendedA, extendedB)
	if err != nil {
		return err
	}
	refs := make([]ast.ASTRef, 8)
	defer func() { result = errors.Join(result, releaseRefs(m, product), releaseRefs(m, refs)) }()
	for i := range refs {
		refs[i], err = m.NewRef(falsehood)
		if err != nil {
			return err
		}
	}
	overflow1, violation, term, adjustedA, adjustedB, accumulatedA, sign, overflow := refs[0], refs[1], refs[2], refs[3], refs[4], refs[5], refs[6], refs[7]
	productBits, err := e.handles(product)
	if err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpXor, overflow1, productBits[len(a)], productBits[len(a)-1]); err != nil {
		return err
	}
	for i := 1; i+1 < len(a); i++ {
		if err = e.config.simplify(ast.ASTOpXor, adjustedB, b[len(b)-1], b[i]); err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpXor, adjustedA, a[len(a)-1], a[len(a)-1-i]); err != nil {
			return err
		}
		adjustedAHandle, err := m.RefGet(adjustedA)
		if err != nil {
			return err
		}
		current, err := m.RefGet(accumulatedA)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpOr, accumulatedA, adjustedAHandle, current); err != nil {
			return err
		}
		accumulatedHandle, err := m.RefGet(accumulatedA)
		if err != nil {
			return err
		}
		adjustedBHandle, err := m.RefGet(adjustedB)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpAnd, term, accumulatedHandle, adjustedBHandle); err != nil {
			return err
		}
		termHandle, err := m.RefGet(term)
		if err != nil {
			return err
		}
		current, err = m.RefGet(violation)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpOr, violation, termHandle, current); err != nil {
			return err
		}
	}
	overflow1Handle, err := m.RefGet(overflow1)
	if err != nil {
		return err
	}
	violationHandle, err := m.RefGet(violation)
	if err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpOr, overflow, overflow1Handle, violationHandle); err != nil {
		return err
	}
	if positiveOverflow {
		err = e.config.simplify(ast.ASTOpIff, sign, a[len(a)-1], b[len(b)-1])
	} else {
		err = e.config.simplify(ast.ASTOpXor, sign, a[len(a)-1], b[len(b)-1])
	}
	if err != nil {
		return err
	}
	signHandle, err := m.RefGet(sign)
	if err != nil {
		return err
	}
	overflowHandle, err := m.RefGet(overflow)
	if err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpAnd, overflow, signHandle, overflowHandle); err != nil {
		return err
	}
	overflowHandle, err = m.RefGet(overflow)
	if err != nil {
		return err
	}
	return e.config.simplify(ast.ASTOpNot, out, overflowHandle)
}

func (e *Engine) MkSignedMulNoOverflow(a, b []ast.ASTHandle, out ast.ASTRef) error {
	return e.signedMulBound(a, b, true, out)
}

func (e *Engine) MkSignedMulNoUnderflow(a, b []ast.ASTHandle, out ast.ASTRef) error {
	return e.signedMulBound(a, b, false, out)
}
