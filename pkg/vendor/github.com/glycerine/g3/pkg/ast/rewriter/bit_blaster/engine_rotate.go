package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:1040-1086.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Engine) extendedRotate(left bool, bits, amountBits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(bits) == 0 || len(bits) != len(amountBits) {
		return nil, ast.ErrASTDomain
	}
	if amount, yes := e.Numeral(amountBits); yes {
		modulus := big.NewInt(int64(len(bits)))
		n := new(big.Int).Mod(amount, modulus).Uint64()
		if left {
			return e.MkRotateLeft(bits, n)
		}
		return e.MkRotateRight(bits, n)
	}
	m := e.config.manager
	widthBits, err := e.NumToBits(big.NewInt(int64(len(bits))), len(bits))
	if err != nil {
		return nil, err
	}
	var masked, equalities []ast.ASTRef
	defer func() {
		result = errors.Join(result, releaseGroups(m, widthBits, masked, equalities))
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	widthHandles, err := e.handles(widthBits)
	if err != nil {
		return nil, err
	}
	masked, err = e.MkUnsignedRem(amountBits, widthHandles)
	if err != nil {
		return nil, err
	}
	maskedHandles, err := e.handles(masked)
	if err != nil {
		return nil, err
	}
	equalities, err = e.MkEqs(maskedHandles)
	if err != nil {
		return nil, err
	}
	equalityHandles, err := e.handles(equalities)
	if err != nil {
		return nil, err
	}
	for index := range bits {
		if err := e.checkpoint(); err != nil {
			return nil, err
		}
		current, err := m.NewRef(bits[index])
		if err != nil {
			return nil, err
		}
		for amount := 1; amount < len(bits); amount++ {
			source := (index + amount) % len(bits)
			if left {
				source = (len(bits) + index - amount) % len(bits)
			}
			currentHandle, err := m.RefGet(current)
			if err != nil {
				_ = m.DelRef(&current)
				return nil, err
			}
			next, err := e.gateRef(ast.ASTOpIte, equalityHandles[amount], bits[source], currentHandle)
			if err != nil {
				_ = m.DelRef(&current)
				return nil, err
			}
			if err = m.DelRef(&current); err != nil {
				_ = m.DelRef(&next)
				return nil, err
			}
			current = next
		}
		out = append(out, current)
	}
	return out, nil
}

func (e *Engine) MkExtendedRotateLeft(bits, amount []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.extendedRotate(true, bits, amount)
}

func (e *Engine) MkExtendedRotateRight(bits, amount []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.extendedRotate(false, bits, amount)
}
