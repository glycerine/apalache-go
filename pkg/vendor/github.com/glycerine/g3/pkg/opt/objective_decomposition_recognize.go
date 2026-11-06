package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *ObjectiveDecomposer) appKind(h ast.ASTHandle, family ast.FamilyID, kind int32) (bool, error) {
	k, err := d.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := d.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := d.manager.GetDeclKind(h)
	return op == kind, err
}

func (d *ObjectiveDecomposer) args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := d.manager.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		result[i], err = d.manager.GetArg(h, i)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (d *ObjectiveDecomposer) rational(h ast.ASTHandle) (value big.Rat, matched bool, err error) {
	text, _, matched, err := d.manager.ArithmeticNumeralText(h)
	if err != nil {
		return big.Rat{}, matched, err
	}
	if matched {
		if _, ok := value.SetString(text); !ok {
			return big.Rat{}, false, ast.ErrASTDomain
		}
		return value, true, nil
	}
	for _, source := range []struct {
		kind   int32
		negate bool
	}{{8, true}, {14, false}} {
		yes, e := d.appKind(h, d.arith, source.kind)
		if e != nil {
			return big.Rat{}, false, e
		}
		if !yes {
			continue
		}
		args, e := d.args(h)
		if e != nil || len(args) != 1 {
			return big.Rat{}, false, e
		}
		value, matched, e = d.rational(args[0])
		if e == nil && matched && source.negate {
			value.Neg(&value)
		}
		return value, matched, e
	}
	return big.Rat{}, false, nil
}
