package fp

// Source karr_relation::is_linear, restricted to the bound integer columns
// admitted by the concrete Datalog guard compiler.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (a *karrDatalogActivation) addKarrLinear(expr ast.ASTHandle, factor *big.Rat, positions map[ast.ASTHandle]uint32, row []big.Rat, constant *big.Rat, integer ast.ASTHandle, arithmetic ast.FamilyID) error {
	sort, err := a.manager.GetSort(expr)
	if err != nil {
		return err
	}
	if sort != integer {
		return ErrKarrDatalogFragment
	}
	if index, ok := positions[expr]; ok {
		row[index].Add(&row[index], factor)
		return nil
	}
	kind, err := a.manager.GetKind(expr)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ErrKarrDatalogFragment
	}
	text, _, matched, err := a.manager.ArithmeticNumeralText(expr)
	if err != nil {
		return err
	}
	if matched {
		value, ok := new(big.Rat).SetString(text)
		if !ok || !value.IsInt() {
			return ErrKarrDatalogFragment
		}
		constant.Add(constant, new(big.Rat).Mul(factor, value))
		return nil
	}
	family, err := a.manager.GetFamilyID(expr)
	if err != nil {
		return err
	}
	if family != arithmetic {
		return ErrKarrDatalogFragment
	}
	op, err := a.manager.GetDeclKind(expr)
	if err != nil {
		return err
	}
	n, err := a.manager.GetNumArgs(expr)
	if err != nil {
		return err
	}
	arg := func(i uint32) (ast.ASTHandle, error) { return a.manager.GetArg(expr, i) }
	switch op {
	case 6: // n-ary addition, in source order
		for i := uint32(0); i < n; i++ {
			term, err := arg(i)
			if err != nil {
				return err
			}
			if err = a.addKarrLinear(term, factor, positions, row, constant, integer, arithmetic); err != nil {
				return err
			}
		}
		return nil
	case 7: // binary subtraction
		if n != 2 {
			return ErrKarrDatalogFragment
		}
		left, err := arg(0)
		if err != nil {
			return err
		}
		right, err := arg(1)
		if err != nil {
			return err
		}
		if err = a.addKarrLinear(left, factor, positions, row, constant, integer, arithmetic); err != nil {
			return err
		}
		return a.addKarrLinear(right, new(big.Rat).Neg(factor), positions, row, constant, integer, arithmetic)
	case 8: // unary minus
		if n != 1 {
			return ErrKarrDatalogFragment
		}
		term, err := arg(0)
		if err != nil {
			return err
		}
		return a.addKarrLinear(term, new(big.Rat).Neg(factor), positions, row, constant, integer, arithmetic)
	case 9: // multiplication with a numeral, checking the left first
		if n != 2 {
			return ErrKarrDatalogFragment
		}
		left, err := arg(0)
		if err != nil {
			return err
		}
		right, err := arg(1)
		if err != nil {
			return err
		}
		text, _, matched, err := a.manager.ArithmeticNumeralText(left)
		if err != nil {
			return err
		}
		term := right
		if !matched {
			text, _, matched, err = a.manager.ArithmeticNumeralText(right)
			if err != nil {
				return err
			}
			term = left
		}
		if !matched {
			return ErrKarrDatalogFragment
		}
		coefficient, ok := new(big.Rat).SetString(text)
		if !ok || !coefficient.IsInt() {
			return ErrKarrDatalogFragment
		}
		return a.addKarrLinear(term, new(big.Rat).Mul(factor, coefficient), positions, row, constant, integer, arithmetic)
	default:
		return ErrKarrDatalogFragment
	}
}
