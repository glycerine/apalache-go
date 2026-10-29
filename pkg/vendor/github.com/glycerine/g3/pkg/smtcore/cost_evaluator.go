package smtcore

// Source: src/smt/cost_evaluator.h and src/smt/cost_evaluator.cpp.

import (
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

const (
	smtArithOpNum int32 = iota
	_
	smtArithOpLE
	smtArithOpGE
	smtArithOpLT
	smtArithOpGT
	smtArithOpAdd
	smtArithOpSub
	smtArithOpUMinus
	smtArithOpMul
	smtArithOpDiv
)

// SMTCostEvaluator evaluates the small expression language used for
// quantifier-instantiation costs. The manager and warning service are borrowed.
type SMTCostEvaluator struct {
	manager  *ast.ASTManager
	warnings *diagnostic.WarningService
	args     []float32
}

func NewSMTCostEvaluator(m *ast.ASTManager, warnings *diagnostic.WarningService) (*SMTCostEvaluator, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	if warnings == nil {
		warnings = diagnostic.BorrowProcessWarningService()
	}
	return &SMTCostEvaluator{manager: m, warnings: warnings}, nil
}

// Eval uses the source de Bruijn convention: variable 0 reads the last arg.
func (e *SMTCostEvaluator) Eval(f ast.ASTHandle, args []float32) (float32, error) {
	if e == nil || e.manager == nil || e.warnings == nil {
		return 0, ast.ErrASTDomain
	}
	e.args = args
	return e.eval(f)
}

func (e *SMTCostEvaluator) arg(f ast.ASTHandle, index uint32) (float32, error) {
	h, err := e.manager.GetArg(f, index)
	if err != nil {
		return 0, err
	}
	return e.eval(h)
}

func (e *SMTCostEvaluator) eval(f ast.ASTHandle) (float32, error) {
	kind, err := e.manager.GetKind(f)
	if err != nil {
		return 0, err
	}
	if kind == ast.ASTVar {
		index, err := e.manager.GetVarIndex(f)
		if err != nil {
			return 0, err
		}
		if index < uint32(len(e.args)) {
			return e.args[len(e.args)-int(index)-1], nil
		}
		return e.fallback("cost function evaluation error")
	}
	if kind != ast.ASTApp {
		return e.fallback("cost function evaluation error")
	}
	family, err := e.manager.GetFamilyID(f)
	if err != nil {
		return 0, err
	}
	op, err := e.manager.GetDeclKind(f)
	if err != nil {
		return 0, err
	}
	if family == e.manager.BasicFamilyID() {
		return e.evalBasic(f, op)
	}
	arithmetic, err := e.manager.ArithmeticFamilyID()
	if err != nil {
		return 0, err
	}
	if family == arithmetic {
		return e.evalArithmetic(f, op)
	}
	return e.fallback("cost function evaluation error")
}

func (e *SMTCostEvaluator) evalBasic(f ast.ASTHandle, op int32) (float32, error) {
	switch op {
	case ast.ASTOpTrue:
		return 1, nil
	case ast.ASTOpFalse:
		return 0, nil
	case ast.ASTOpNot:
		v, err := e.arg(f, 0)
		if err != nil || v == 0 {
			return 1, err
		}
		return 0, nil
	case ast.ASTOpAnd, ast.ASTOpOr:
		n, err := e.manager.GetNumArgs(f)
		if err != nil {
			return 0, err
		}
		for i := uint32(0); i < n; i++ {
			v, err := e.arg(f, i)
			if err != nil {
				return 0, err
			}
			if op == ast.ASTOpAnd && v == 0 {
				return 0, nil
			}
			if op == ast.ASTOpOr && v != 0 {
				return 1, nil
			}
		}
		if op == ast.ASTOpAnd {
			return 1, nil
		}
		return 0, nil
	case ast.ASTOpIte:
		condition, err := e.arg(f, 0)
		if err != nil {
			return 0, err
		}
		if condition != 0 {
			return e.arg(f, 1)
		}
		return e.arg(f, 2)
	case ast.ASTOpEq, ast.ASTOpIff, ast.ASTOpXor:
		left, err := e.arg(f, 0)
		if err != nil {
			return 0, err
		}
		right, err := e.arg(f, 1)
		if err != nil {
			return 0, err
		}
		equal := left == right
		if op == ast.ASTOpXor {
			equal = !equal
		}
		if equal {
			return 1, nil
		}
		return 0, nil
	case ast.ASTOpImplies:
		left, err := e.arg(f, 0)
		if err != nil {
			return 0, err
		}
		if left == 0 {
			return 1, nil
		}
		right, err := e.arg(f, 1)
		if err != nil {
			return 0, err
		}
		if right != 0 {
			return 1, nil
		}
		return 0, nil
	default:
		return e.fallback("cost function evaluation error")
	}
}

func (e *SMTCostEvaluator) evalArithmetic(f ast.ASTHandle, op int32) (float32, error) {
	if op == smtArithOpNum {
		text, _, matched, err := e.manager.ArithmeticNumeralText(f)
		if err != nil {
			return 0, err
		}
		if !matched {
			return e.fallback("cost function evaluation error")
		}
		return smtCostNumeral(text)
	}
	left, err := e.arg(f, 0)
	if err != nil {
		return 0, err
	}
	if op == smtArithOpUMinus {
		return -left, nil
	}
	right, err := e.arg(f, 1)
	if err != nil {
		return 0, err
	}
	switch op {
	case smtArithOpLE:
		return boolCost(left <= right), nil
	case smtArithOpGE:
		return boolCost(left >= right), nil
	case smtArithOpLT:
		return boolCost(left < right), nil
	case smtArithOpGT:
		return boolCost(left > right), nil
	case smtArithOpAdd:
		return left + right, nil
	case smtArithOpSub:
		return left - right, nil
	case smtArithOpMul:
		return left * right, nil
	case smtArithOpDiv:
		if right == 0 {
			return e.fallback("cost function division by zero")
		}
		return left / right, nil
	default:
		return e.fallback("cost function evaluation error")
	}
}

func smtCostNumeral(text string) (float32, error) {
	numerator, denominator := text, "1"
	if slash := strings.IndexByte(text, '/'); slash >= 0 {
		numerator, denominator = text[:slash], text[slash+1:]
	}
	n, err := strconv.ParseFloat(numerator, 32)
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(denominator, 32)
	if err != nil {
		return 0, err
	}
	return float32(n) / float32(d), nil
}

func boolCost(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func (e *SMTCostEvaluator) fallback(message string) (float32, error) {
	if err := e.warnings.Warning(message); err != nil {
		return 0, err
	}
	return 1, nil
}
