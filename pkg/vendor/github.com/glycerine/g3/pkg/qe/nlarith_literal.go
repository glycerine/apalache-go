package qe

import "github.com/glycerine/g3/pkg/ast"

type NLComparison uint8

const (
	NLComparisonLE NLComparison = iota
	NLComparisonLT
	NLComparisonEQ
	NLComparisonNE
)

type NLLiteral struct {
	Atom       ast.ASTRef
	Polynomial *NLPolynomial
	Comparison NLComparison
}

func (l *NLLiteral) Close() error {
	if l == nil || l.Polynomial == nil {
		return ast.ErrASTHandle
	}
	m := l.Polynomial.manager
	err1 := l.Polynomial.Close()
	err2 := m.DelRef(&l.Atom)
	if err1 != nil {
		return err1
	}
	return err2
}

func (u *NonlinearArithmeticUtil) parseRelation(expression ast.ASTHandle) (left, right ast.ASTHandle, comparison NLComparison, matched bool, err error) {
	inner := expression
	negative := false
	if args, ok, cause := u.term.operation(expression, u.manager.BasicFamilyID(), ast.ASTOpNot); cause != nil {
		return left, right, 0, false, cause
	} else if ok && len(args) == 1 {
		inner = args[0]
		negative = true
	}
	family, cause := u.manager.GetFamilyID(inner)
	if cause != nil {
		return left, right, 0, false, cause
	}
	kind, cause := u.manager.GetDeclKind(inner)
	if cause != nil {
		return left, right, 0, false, cause
	}
	if family == u.family && (kind >= 2 && kind <= 5) {
		left, cause = u.manager.GetArg(inner, 0)
		if cause == nil {
			right, cause = u.manager.GetArg(inner, 1)
		}
		if cause != nil {
			return left, right, 0, false, cause
		}
		if kind == 3 || kind == 5 {
			left, right = right, left
		}
		if kind == 2 || kind == 3 {
			comparison = NLComparisonLE
			if negative {
				left, right = right, left
				comparison = NLComparisonLT
			}
		} else {
			comparison = NLComparisonLT
			if negative {
				left, right = right, left
				comparison = NLComparisonLE
			}
		}
		return left, right, comparison, true, nil
	}
	if family == u.manager.BasicFamilyID() && kind == ast.ASTOpEq {
		left, cause = u.manager.GetArg(inner, 0)
		if cause == nil {
			right, cause = u.manager.GetArg(inner, 1)
		}
		if cause != nil {
			return left, right, 0, false, cause
		}
		comparison = NLComparisonEQ
		if negative {
			comparison = NLComparisonNE
		}
		return left, right, comparison, true, nil
	}
	return left, right, 0, false, nil
}

func (u *NonlinearArithmeticUtil) ParseLiteral(variable *ast.ContainsApp, expression ast.ASTHandle) (*NLLiteral, bool, error) {
	left, right, comparison, ok, err := u.parseRelation(expression)
	if err != nil || !ok {
		return nil, ok, err
	}
	difference, err := u.manager.MkArithmeticSub(left, right)
	if err != nil {
		return nil, false, err
	}
	p, ok, err := u.Decompose(variable, difference)
	if err != nil || !ok {
		return nil, ok, err
	}
	r, err := u.manager.NewRef(expression)
	if err != nil {
		_ = p.Close()
		return nil, false, err
	}
	return &NLLiteral{r, p, comparison}, true, nil
}
