package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:1088-1120.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Engine) mkLE(signed bool, a, b []ast.ASTHandle, out ast.ASTRef) (result error) {
	if len(a) == 0 || len(a) != len(b) {
		return ast.ErrASTDomain
	}
	m := e.config.manager
	temps := make([]ast.ASTRef, 4, 5)
	for i := range temps {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return errors.Join(err, releaseRefs(m, temps[:i]))
		}
		temps[i] = ref
	}
	defer func() { result = errors.Join(result, releaseRefs(m, temps)) }()
	notA, i1, i2, i3 := temps[0], temps[1], temps[2], temps[3]
	if err := e.config.simplify(ast.ASTOpNot, notA, a[0]); err != nil {
		return err
	}
	notAHandle, err := m.RefGet(notA)
	if err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpOr, out, notAHandle, b[0]); err != nil {
		return err
	}
	limit := len(a)
	if signed {
		limit--
	}
	for index := 1; index < limit; index++ {
		if err = e.config.simplify(ast.ASTOpNot, notA, a[index]); err != nil {
			return err
		}
		notAHandle, err = m.RefGet(notA)
		if err != nil {
			return err
		}
		previous, err := m.RefGet(out)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpAnd, i1, notAHandle, b[index]); err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpAnd, i2, notAHandle, previous); err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpAnd, i3, b[index], previous); err != nil {
			return err
		}
		h1, err := m.RefGet(i1)
		if err != nil {
			return err
		}
		h2, err := m.RefGet(i2)
		if err != nil {
			return err
		}
		h3, err := m.RefGet(i3)
		if err != nil {
			return err
		}
		if err = e.config.simplify(ast.ASTOpOr, out, h1, h2, h3); err != nil {
			return err
		}
	}
	if !signed {
		return nil
	}
	notB, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	temps = append(temps, notB)
	most := len(a) - 1
	if err = e.config.simplify(ast.ASTOpNot, notB, b[most]); err != nil {
		return err
	}
	notBHandle, err := m.RefGet(notB)
	if err != nil {
		return err
	}
	previous, err := m.RefGet(out)
	if err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpAnd, i1, notBHandle, a[most]); err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpAnd, i2, notBHandle, previous); err != nil {
		return err
	}
	if err = e.config.simplify(ast.ASTOpAnd, i3, a[most], previous); err != nil {
		return err
	}
	h1, err := m.RefGet(i1)
	if err != nil {
		return err
	}
	h2, err := m.RefGet(i2)
	if err != nil {
		return err
	}
	h3, err := m.RefGet(i3)
	if err != nil {
		return err
	}
	return e.config.simplify(ast.ASTOpOr, out, h1, h2, h3)
}

func (e *Engine) MkUnsignedLE(a, b []ast.ASTHandle, out ast.ASTRef) error {
	return e.mkLE(false, a, b, out)
}

func (e *Engine) MkSignedLE(a, b []ast.ASTHandle, out ast.ASTRef) error {
	return e.mkLE(true, a, b, out)
}
