package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:439-509.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Engine) MkUnsignedDivRem(a, b []ast.ASTHandle) (quotient, remainder []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, nil, ast.ErrASTDomain
	}
	m := e.config.manager
	width := len(a)
	quotient = make([]ast.ASTRef, width)
	partial := make([]ast.ASTRef, 0, width)
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(m, quotient), releaseRefs(m, partial))
			quotient, remainder = nil, nil
		}
	}()
	most, err := m.NewRef(a[width-1])
	if err != nil {
		return nil, nil, err
	}
	partial = append(partial, most)
	falsehood, err := m.MkFalse()
	if err != nil {
		return nil, nil, err
	}
	for len(partial) < width {
		ref, err := m.NewRef(falsehood)
		if err != nil {
			return nil, nil, err
		}
		partial = append(partial, ref)
	}

	for stage := 0; stage < width; stage++ {
		if err := e.checkpoint(); err != nil {
			return nil, nil, err
		}
		partialBits, err := e.handles(partial)
		if err != nil {
			return nil, nil, err
		}
		difference, carry, err := e.MkSubtracter(partialBits, b)
		if err != nil {
			return nil, nil, err
		}
		quotient[width-stage-1] = carry
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			_ = releaseRefs(m, difference)
			return nil, nil, err
		}
		differenceBits, err := e.handles(difference)
		if err != nil {
			_ = releaseRefs(m, difference)
			return nil, nil, err
		}
		next := make([]ast.ASTRef, 0, width)
		if stage+1 < width {
			low, err := m.NewRef(a[width-stage-2])
			if err != nil {
				_ = releaseRefs(m, difference)
				return nil, nil, err
			}
			next = append(next, low)
			for index := 1; index < width; index++ {
				ref, err := e.gateRef(ast.ASTOpIte, carryHandle, differenceBits[index-1], partialBits[index-1])
				if err != nil {
					_ = releaseRefs(m, next)
					_ = releaseRefs(m, difference)
					return nil, nil, err
				}
				next = append(next, ref)
			}
		} else {
			for index := 0; index < width; index++ {
				ref, err := e.gateRef(ast.ASTOpIte, carryHandle, differenceBits[index], partialBits[index])
				if err != nil {
					_ = releaseRefs(m, next)
					_ = releaseRefs(m, difference)
					return nil, nil, err
				}
				next = append(next, ref)
			}
		}
		if err = releaseRefs(m, difference); err != nil {
			_ = releaseRefs(m, next)
			return nil, nil, err
		}
		if err = releaseRefs(m, partial); err != nil {
			_ = releaseRefs(m, next)
			return nil, nil, err
		}
		partial = next
	}
	remainder = partial
	partial = nil
	return quotient, remainder, nil
}

func (e *Engine) MkUnsignedDiv(a, b []ast.ASTHandle) (quotient []ast.ASTRef, result error) {
	quotient, remainder, err := e.MkUnsignedDivRem(a, b)
	if err != nil {
		return nil, err
	}
	if err = releaseRefs(e.config.manager, remainder); err != nil {
		return nil, errors.Join(err, releaseRefs(e.config.manager, quotient))
	}
	return quotient, nil
}

func (e *Engine) MkUnsignedRem(a, b []ast.ASTHandle) (remainder []ast.ASTRef, result error) {
	quotient, remainder, err := e.MkUnsignedDivRem(a, b)
	if err != nil {
		return nil, err
	}
	if err = releaseRefs(e.config.manager, quotient); err != nil {
		return nil, errors.Join(err, releaseRefs(e.config.manager, remainder))
	}
	return remainder, nil
}
