package sls

// Source: src/tactic/sls/sls_evaluator.h:117-493,523-616.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// bv_op_kind values from src/ast/bv_decl_plugin.h. The AST package deliberately
// exposes theory identities as int32, so this retained consumer names its live
// cases locally instead of introducing a universal enum adapter.
const (
	slsBVNum int32 = iota
	slsBVBit1
	slsBVBit0
	slsBVNeg
	slsBVAdd
	slsBVSub
	slsBVMul
	slsBVSDiv
	slsBVUDiv
	slsBVSRem
	slsBVURem
	slsBVSMod
	slsBVSDiv0
	slsBVUDiv0
	slsBVSRem0
	slsBVURem0
	slsBVSMod0
	slsBVSDivI
	slsBVUDivI
	slsBVSRemI
	slsBVURemI
	slsBVSModI
	slsBVULE
	slsBVSLE
	slsBVUGE
	slsBVSGE
	slsBVULT
	slsBVSLT
	slsBVUGT
	slsBVSGT
	slsBVAnd
	slsBVOr
	slsBVNot
	slsBVXor
	slsBVNand
	slsBVNor
	slsBVXnor
	slsBVConcat
	slsBVSignExt
	slsBVZeroExt
	slsBVExtract
	slsBVRepeat
	slsBVRedOr
	slsBVRedAnd
	slsBVComp
	slsBVShl
	slsBVLShr
	slsBVAShr
	slsBVRotateLeft
	slsBVRotateRight
	slsBVExtRotateLeft
	slsBVExtRotateRight
	slsBVUMulNoOverflow
	slsBVSMulNoOverflow
	slsBVSMulNoUnderflow
	slsBVBit2Bool
)

type slsArgValue func(uint32) (*numeral.Int, error)

func (e *Evaluator) evaluateBV(node ast.ASTHandle, n uint32, op int32, arg slsArgValue, out *numeral.Int) (result error) {
	owned := make([]*numeral.Int, 0, 4)
	temporary := func() *numeral.Int {
		v := new(numeral.Int)
		owned = append(owned, v)
		return v
	}
	copyTemporary := func(v *numeral.Int) (*numeral.Int, error) {
		t := temporary()
		return t, e.numbers.Copy(t, v)
	}
	defer func() {
		for i := len(owned); i > 0; i-- {
			result = errors.Join(result, e.numbers.Del(owned[i-1]))
		}
	}()
	require := func(want uint32) error {
		if n != want {
			return ast.ErrASTDomain
		}
		return nil
	}
	width := func(h ast.ASTHandle) (uint32, error) { return e.manager.BitVectorSize(h) }
	argAt := func(i uint32) (ast.ASTHandle, *numeral.Int, error) {
		h, x := e.manager.GetArg(node, i)
		if x != nil {
			return ast.ASTHandle{}, nil, x
		}
		v, x := arg(i)
		return h, v, x
	}
	setBoolean := func(value bool) error {
		if value {
			return e.numbers.Copy(out, &e.one)
		}
		return e.numbers.Copy(out, &e.zero)
	}
	signed := func(v *numeral.Int, bits uint32) error {
		if bits == 0 {
			return ast.ErrASTDomain
		}
		power, x := e.powers.borrow(bits)
		if x != nil {
			return x
		}
		half, x := e.powers.borrow(bits - 1)
		if x != nil {
			return x
		}
		atLeast, x := e.numbers.GreaterEqual(v, half)
		if x != nil || !atLeast {
			return x
		}
		return e.numbers.Sub(v, power, v)
	}

	switch op {
	case slsBVConcat:
		if n < 2 {
			return ast.ErrASTDomain
		}
		for i := uint32(0); i < n; i++ {
			h, value, x := argAt(i)
			if x != nil {
				return x
			}
			if i != 0 {
				bits, x := width(h)
				if x != nil {
					return x
				}
				power, x := e.powers.borrow(bits)
				if x != nil {
					return x
				}
				if x = e.numbers.Mul(out, power, out); x != nil {
					return x
				}
			}
			if x = e.numbers.Add(out, value, out); x != nil {
				return x
			}
		}
	case slsBVExtract:
		if x := require(1); x != nil {
			return x
		}
		_, child, x := argAt(0)
		if x != nil {
			return x
		}
		high, x := e.manager.BitVectorExtractHigh(node)
		if x != nil {
			return x
		}
		low, x := e.manager.BitVectorExtractLow(node)
		if x != nil {
			return x
		}
		power, x := e.powers.borrow(high + 1)
		if x != nil {
			return x
		}
		if x = e.numbers.Rem(child, power, out); x != nil {
			return x
		}
		if x = e.numbers.MachineDiv2kInPlace(out, low); x != nil {
			return x
		}
	case slsBVAdd:
		if n < 2 {
			return ast.ErrASTDomain
		}
		for i := uint32(0); i < n; i++ {
			value, x := arg(i)
			if x != nil {
				return x
			}
			if x = e.numbers.Add(out, value, out); x != nil {
				return x
			}
		}
		bits, x := width(node)
		if x != nil {
			return x
		}
		power, x := e.powers.borrow(bits)
		if x != nil {
			return x
		}
		if x = e.numbers.Rem(out, power, out); x != nil {
			return x
		}
	case slsBVSub:
		if x := require(2); x != nil {
			return x
		}
		left, x := arg(0)
		if x != nil {
			return x
		}
		right, x := arg(1)
		if x != nil {
			return x
		}
		temp := temporary()
		if x = e.numbers.Sub(left, right, temp); x != nil {
			return x
		}
		bits, x := width(node)
		if x != nil {
			return x
		}
		power, x := e.powers.borrow(bits)
		if x != nil {
			return x
		}
		if x = e.numbers.Mod(temp, power, out); x != nil {
			return x
		}
	case slsBVMul:
		if n < 2 {
			return ast.ErrASTDomain
		}
		first, x := arg(0)
		if x != nil {
			return x
		}
		if x = e.numbers.Copy(out, first); x != nil {
			return x
		}
		for i := uint32(1); i < n; i++ {
			value, x := arg(i)
			if x != nil {
				return x
			}
			if x = e.numbers.Mul(out, value, out); x != nil {
				return x
			}
		}
		bits, x := width(node)
		if x != nil {
			return x
		}
		power, x := e.powers.borrow(bits)
		if x != nil {
			return x
		}
		if x = e.numbers.Rem(out, power, out); x != nil {
			return x
		}
	case slsBVNeg:
		if x := require(1); x != nil {
			return x
		}
		value, x := arg(0)
		if x != nil {
			return x
		}
		zero, x := e.numbers.IsZero(value)
		if x != nil {
			return x
		}
		if zero {
			return e.numbers.Copy(out, &e.zero)
		}
		bits, x := width(node)
		if x != nil {
			return x
		}
		if x = e.numbers.BitwiseNot(bits, value, out); x != nil {
			return x
		}
		if x = e.numbers.Inc(out); x != nil {
			return x
		}
	case slsBVSDiv, slsBVSDiv0, slsBVSDivI, slsBVUDiv, slsBVUDiv0, slsBVUDivI,
		slsBVSRem, slsBVSRem0, slsBVSRemI, slsBVURem, slsBVURem0, slsBVURemI,
		slsBVSMod, slsBVSMod0, slsBVSModI:
		if x := require(2); x != nil {
			return x
		}
		left, x := arg(0)
		if x != nil {
			return x
		}
		right, x := arg(1)
		if x != nil {
			return x
		}
		tx, xerr := copyTemporary(left)
		if xerr != nil {
			return xerr
		}
		ty, xerr := copyTemporary(right)
		if xerr != nil {
			return xerr
		}
		bits, xerr := width(node)
		if xerr != nil {
			return xerr
		}
		power, xerr := e.powers.borrow(bits)
		if xerr != nil {
			return xerr
		}
		signedOp := op == slsBVSDiv || op == slsBVSDiv0 || op == slsBVSDivI || op == slsBVSRem || op == slsBVSRem0 || op == slsBVSRemI || op == slsBVSMod || op == slsBVSMod0 || op == slsBVSModI
		if signedOp {
			if xerr = signed(tx, bits); xerr != nil {
				return xerr
			}
			if xerr = signed(ty, bits); xerr != nil {
				return xerr
			}
		}
		yZero, xerr := e.numbers.IsZero(ty)
		if xerr != nil {
			return xerr
		}
		switch op {
		case slsBVSDiv, slsBVSDiv0, slsBVSDivI:
			if yZero {
				negative, qerr := e.numbers.IsNeg(tx)
				if qerr != nil {
					return qerr
				}
				if negative {
					xerr = e.numbers.Copy(out, &e.one)
				} else if xerr = e.numbers.Copy(out, power); xerr == nil {
					xerr = e.numbers.Dec(out)
				}
			} else {
				xerr = e.numbers.MachineDiv(tx, ty, out)
			}
		case slsBVUDiv, slsBVUDiv0, slsBVUDivI:
			if yZero {
				if xerr = e.numbers.Copy(out, power); xerr == nil {
					xerr = e.numbers.Dec(out)
				}
			} else {
				xerr = e.numbers.MachineDiv(tx, ty, out)
			}
		case slsBVSRem, slsBVSRem0, slsBVSRemI:
			if yZero {
				xerr = e.numbers.Copy(out, tx)
			} else {
				xerr = e.numbers.Rem(tx, ty, out)
			}
		case slsBVURem, slsBVURem0, slsBVURemI:
			if yZero {
				xerr = e.numbers.Copy(out, tx)
			} else {
				xerr = e.numbers.Mod(tx, ty, out)
			}
		default:
			if yZero {
				xerr = e.numbers.Copy(out, tx)
			} else {
				negX, z := e.numbers.IsNeg(tx)
				if z != nil {
					return z
				}
				negY, z := e.numbers.IsNeg(ty)
				if z != nil {
					return z
				}
				absX, z := copyTemporary(tx)
				if z != nil {
					return z
				}
				absY, z := copyTemporary(ty)
				if z != nil {
					return z
				}
				if negX {
					if z = e.numbers.Neg(absX); z != nil {
						return z
					}
				}
				if negY {
					if z = e.numbers.Neg(absY); z != nil {
						return z
					}
				}
				if xerr = e.numbers.Mod(absX, absY, out); xerr == nil {
					zero, z := e.numbers.IsZero(out)
					if z != nil {
						return z
					}
					if !zero && (negX || negY) {
						switch {
						case negX && !negY:
							if xerr = e.numbers.Neg(out); xerr == nil {
								xerr = e.numbers.Add(out, ty, out)
							}
						case !negX && negY:
							xerr = e.numbers.Add(out, ty, out)
						default:
							xerr = e.numbers.Neg(out)
						}
					}
				}
			}
		}
		if xerr != nil {
			return xerr
		}
		negative, xerr := e.numbers.IsNeg(out)
		if xerr != nil {
			return xerr
		}
		if negative {
			if xerr = e.numbers.Add(out, power, out); xerr != nil {
				return xerr
			}
		}
	case slsBVAnd, slsBVOr, slsBVXor, slsBVNand, slsBVNor:
		if n < 2 {
			return ast.ErrASTDomain
		}
		first, x := arg(0)
		if x != nil {
			return x
		}
		if x = e.numbers.Copy(out, first); x != nil {
			return x
		}
		bits, x := width(node)
		if x != nil {
			return x
		}
		for i := uint32(1); i < n; i++ {
			value, z := arg(i)
			if z != nil {
				return z
			}
			switch op {
			case slsBVAnd, slsBVNand:
				z = e.numbers.BitwiseAnd(out, value, out)
			case slsBVOr, slsBVNor:
				z = e.numbers.BitwiseOr(out, value, out)
			default:
				z = e.numbers.BitwiseXor(out, value, out)
			}
			if z != nil {
				return z
			}
			if op == slsBVNand || op == slsBVNor {
				if z = e.numbers.BitwiseNot(bits, out, out); z != nil {
					return z
				}
			}
		}
	case slsBVNot:
		if x := require(1); x != nil {
			return x
		}
		h, value, x := argAt(0)
		if x != nil {
			return x
		}
		bits, x := width(h)
		if x != nil {
			return x
		}
		if x = e.numbers.BitwiseNot(bits, value, out); x != nil {
			return x
		}
	case slsBVULT, slsBVULE, slsBVUGT, slsBVUGE, slsBVSLT, slsBVSLE, slsBVSGT, slsBVSGE:
		if x := require(2); x != nil {
			return x
		}
		left, x := arg(0)
		if x != nil {
			return x
		}
		right, x := arg(1)
		if x != nil {
			return x
		}
		var a, b *numeral.Int
		if op == slsBVSLT || op == slsBVSLE || op == slsBVSGT || op == slsBVSGE {
			a, x = copyTemporary(left)
			if x != nil {
				return x
			}
			b, x = copyTemporary(right)
			if x != nil {
				return x
			}
			h, _, z := argAt(0)
			if z != nil {
				return z
			}
			bits, z := width(h)
			if z != nil {
				return z
			}
			if z = signed(a, bits); z != nil {
				return z
			}
			if z = signed(b, bits); z != nil {
				return z
			}
		} else {
			a, b = left, right
		}
		var yes bool
		switch op {
		case slsBVULT, slsBVSLT:
			yes, x = e.numbers.Less(a, b)
		case slsBVULE, slsBVSLE:
			yes, x = e.numbers.LessEqual(a, b)
		case slsBVUGT, slsBVSGT:
			yes, x = e.numbers.Greater(a, b)
		default:
			yes, x = e.numbers.GreaterEqual(a, b)
		}
		if x != nil {
			return x
		}
		if x = setBoolean(yes); x != nil {
			return x
		}
	case slsBVBit2Bool, slsBVSignExt:
		if x := require(1); x != nil {
			return x
		}
		value, x := arg(0)
		if x != nil {
			return x
		}
		if x = e.numbers.Copy(out, value); x != nil {
			return x
		}
	case slsBVAShr, slsBVLShr, slsBVShl:
		if x := require(2); x != nil {
			return x
		}
		h, value, x := argAt(0)
		if x != nil {
			return x
		}
		shiftValue, x := arg(1)
		if x != nil {
			return x
		}
		if x = e.numbers.Copy(out, value); x != nil {
			return x
		}
		shift, x := copyTemporary(shiftValue)
		if x != nil {
			return x
		}
		var sign *numeral.Int
		var divisionTemp *numeral.Int
		if op == slsBVAShr {
			bits, z := width(h)
			if z != nil || bits == 0 {
				if z != nil {
					return z
				}
				return ast.ErrASTDomain
			}
			mask, z := e.powers.borrow(bits - 1)
			if z != nil {
				return z
			}
			sign = temporary()
			divisionTemp = temporary()
			if z = e.numbers.BitwiseAnd(out, mask, sign); z != nil {
				return z
			}
		}
		for {
			zero, z := e.numbers.IsZero(shift)
			if z != nil {
				return z
			}
			if zero {
				break
			}
			switch op {
			case slsBVAShr:
				if z = e.numbers.MachineDiv(out, &e.two, divisionTemp); z == nil {
					z = e.numbers.Add(divisionTemp, sign, out)
				}
			case slsBVLShr:
				z = e.numbers.MachineDiv(out, &e.two, out)
			default:
				z = e.numbers.Mul(out, &e.two, out)
			}
			if z != nil {
				return z
			}
			if z = e.numbers.Dec(shift); z != nil {
				return z
			}
		}
		if op == slsBVShl {
			bits, z := width(node)
			if z != nil {
				return z
			}
			power, z := e.powers.borrow(bits)
			if z != nil {
				return z
			}
			if z = e.numbers.Rem(out, power, out); z != nil {
				return z
			}
		}
	default:
		return ErrEvaluatorUnsupported
	}
	nonnegative, err := e.numbers.IsNonNeg(out)
	if err != nil {
		return err
	}
	if !nonnegative {
		return ErrTracker
	}
	return nil
}

// EvaluateGraph follows the initialized app graph in post-order and publishes
// every derived value into its existing value_score entry.
func (e *Evaluator) EvaluateGraph(roots []ast.ASTHandle) (result error) {
	if err := e.open(); err != nil {
		return err
	}
	visited := make(map[ast.ASTHandle]struct{})
	temp := numeral.NewInt(0)
	defer func() { result = errors.Join(result, e.numbers.Del(&temp)) }()
	var visit func(ast.ASTHandle) error
	visit = func(node ast.ASTHandle) error {
		if _, found := visited[node]; found {
			return nil
		}
		visited[node] = struct{}{}
		if _, err := e.tracker.existingScore(node); err != nil {
			return err
		}
		n, err := e.manager.GetNumArgs(node)
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			child, x := e.manager.GetArg(node, i)
			if x != nil {
				return x
			}
			if x = visit(child); x != nil {
				return x
			}
		}
		if n == 0 {
			return nil
		}
		if err = e.Evaluate(node, &temp); err != nil {
			return err
		}
		return e.tracker.SetValue(node, &temp)
	}
	for _, root := range roots {
		if err := visit(root); err != nil {
			return err
		}
	}
	return nil
}
