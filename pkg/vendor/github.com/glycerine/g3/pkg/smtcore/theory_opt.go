package smtcore

// Source: src/smt/theory_opt.cpp is_linear() and is_numeral().

import "github.com/glycerine/g3/pkg/ast"

const (
	smtArithOpToReal int32 = 14
	smtArithOpToInt  int32 = 15
)

// SMTTheoryOptimizationUtil owns no ASTs. It borrows the manager used to
// recognize the arithmetic expression fragment accepted by theory optimizers.
type SMTTheoryOptimizationUtil struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
}

func NewSMTTheoryOptimizationUtil(manager *ast.ASTManager) (*SMTTheoryOptimizationUtil, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return &SMTTheoryOptimizationUtil{manager: manager, arithmetic: family}, nil
}

func (u *SMTTheoryOptimizationUtil) IsNumeral(term ast.ASTHandle) (bool, error) {
	if u == nil || u.manager == nil {
		return false, ast.ErrASTHandle
	}
	for {
		kind, err := u.manager.GetKind(term)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		family, err := u.manager.GetFamilyID(term)
		if err != nil {
			return false, err
		}
		if family != u.arithmetic {
			return false, nil
		}
		op, err := u.manager.GetDeclKind(term)
		if err != nil {
			return false, err
		}
		switch op {
		case smtArithOpUMinus, smtArithOpToReal, smtArithOpToInt:
			term, err = u.manager.GetArg(term, 0)
			if err != nil {
				return false, err
			}
		case smtArithOpNum:
			return true, nil
		default:
			return false, nil
		}
	}
}

func (u *SMTTheoryOptimizationUtil) IsLinear(term ast.ASTHandle) (bool, error) {
	if u == nil || u.manager == nil {
		return false, ast.ErrASTHandle
	}
	todo := []ast.ASTHandle{term}
	marked := make(map[ast.ASTHandle]struct{})
	for len(todo) != 0 {
		index := len(todo) - 1
		current := todo[index]
		todo = todo[:index]
		if _, found := marked[current]; found {
			continue
		}
		marked[current] = struct{}{}
		kind, err := u.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		family, err := u.manager.GetFamilyID(current)
		if err != nil {
			return false, err
		}
		if family != u.arithmetic {
			continue
		}
		op, err := u.manager.GetDeclKind(current)
		if err != nil {
			return false, err
		}
		switch op {
		case smtArithOpAdd, smtArithOpSub, smtArithOpUMinus, smtArithOpNum, smtArithOpToReal, smtArithOpToInt:
			count, e := u.manager.GetNumArgs(current)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < count; i++ {
				argument, e := u.manager.GetArg(current, i)
				if e != nil {
					return false, e
				}
				todo = append(todo, argument)
			}
		case smtArithOpMul:
			first, e := u.manager.GetArg(current, 0)
			if e != nil {
				return false, e
			}
			second, e := u.manager.GetArg(current, 1)
			if e != nil {
				return false, e
			}
			firstNumeral, e := u.IsNumeral(first)
			if e != nil {
				return false, e
			}
			if firstNumeral {
				todo = append(todo, second)
				continue
			}
			secondNumeral, e := u.IsNumeral(second)
			if e != nil {
				return false, e
			}
			if !secondNumeral {
				return false, nil
			}
			todo = append(todo, first)
		default:
			return false, nil
		}
	}
	return true, nil
}
