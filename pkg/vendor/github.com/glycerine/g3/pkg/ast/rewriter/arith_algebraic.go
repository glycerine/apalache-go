package rewriter

// Source: src/ast/rewriter/arith_rewriter.cpp:367-454,561-583,685-733.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (r *ArithRewriter) algebraicNumeralSyntax(h ast.ASTHandle) (bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != r.family {
		return false, err
	}
	declKind, err := r.manager.GetDeclKind(h)
	return err == nil && declKind == 1, err
}

func (r *ArithRewriter) algebraicOperand(h ast.ASTHandle, manager *polynomial.AlgebraicManager) (value polynomial.Algebraic, irrational, matched bool, err error) {
	matched, err = r.manager.ArithmeticAlgebraicNumeralValue(h, &value)
	if err != nil || matched {
		if err == nil {
			degree, e := manager.Degree(&value)
			if e != nil {
				err = e
			} else if degree > r.maxDegree {
				_ = manager.Del(&value)
				value = polynomial.Algebraic{}
				matched = false
				return
			}
		}
		irrational = matched
		return
	}
	rational, _, numeral, e := r.numeral(h)
	if e != nil || !numeral {
		return value, false, false, e
	}
	service := r.manager.NumeralService()
	owned, e := service.NewValue()
	if e != nil {
		return value, false, false, e
	}
	defer func() { err = errors.Join(err, service.Del(&owned)) }()
	if e = service.SetString(&owned, rational.RatString()); e != nil {
		return value, false, false, e
	}
	if e = manager.SetRational(&value, &owned.Raw); e != nil {
		return value, false, false, e
	}
	return value, false, true, nil
}

func (r *ArithRewriter) makeAlgebraicNumeral(manager *polynomial.AlgebraicManager, value *polynomial.Algebraic) (result ast.ASTHandle, err error) {
	rational, err := manager.IsRational(value)
	if err != nil {
		return result, err
	}
	if !rational {
		return r.manager.ArithmeticAlgebraicNumeral(value)
	}
	service := r.manager.NumeralService()
	owned, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, service.Del(&owned)) }()
	if err = manager.ToOwnedRational(value, service, &owned); err != nil {
		return result, err
	}
	return r.manager.ArithmeticNumeral(&owned, false)
}

func (r *ArithRewriter) rewriteAlgebraicComparison(left, right ast.ASTHandle, relation int32, out ast.ASTRef) (decided bool, err error) {
	if !r.algebraicSimplify {
		return false, nil
	}
	leftSyntax, err := r.algebraicNumeralSyntax(left)
	if err != nil {
		return false, err
	}
	rightSyntax, err := r.algebraicNumeralSyntax(right)
	if err != nil || !leftSyntax && !rightSyntax {
		return false, err
	}
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return false, err
	}
	leftValue, leftIrrational, leftMatched, err := r.algebraicOperand(left, manager)
	if err != nil || !leftMatched {
		return false, err
	}
	defer func() { err = errors.Join(err, manager.Del(&leftValue)) }()
	rightValue, rightIrrational, rightMatched, err := r.algebraicOperand(right, manager)
	if err != nil || !rightMatched {
		return false, err
	}
	defer func() { err = errors.Join(err, manager.Del(&rightValue)) }()
	if !leftIrrational && !rightIrrational {
		return false, nil
	}
	comparison, err := manager.Compare(&leftValue, &rightValue)
	if err != nil {
		return false, err
	}
	truth := comparison == 0
	if relation == arithOpLE {
		truth = comparison <= 0
	} else if relation == arithOpGE {
		truth = comparison >= 0
	}
	return true, r.setBool(out, truth)
}

func (r *ArithRewriter) rewriteAlgebraicFold(kind int32, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if !r.algebraicSimplify || args.Count() < 2 {
		return RewriteFailed, nil
	}
	hasAlgebraic := false
	for i := uint32(0); i < args.Count(); i++ {
		h, err := args.At(i)
		if err != nil {
			return RewriteFailed, err
		}
		yes, err := r.algebraicNumeralSyntax(h)
		if err != nil {
			return RewriteFailed, err
		}
		hasAlgebraic = hasAlgebraic || yes
	}
	if !hasAlgebraic {
		return RewriteFailed, nil
	}
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return RewriteFailed, err
	}
	values := make([]polynomial.Algebraic, args.Count())
	constructed := 0
	irrationals, rationals := 0, 0
	defer func() {
		for i := 0; i < constructed; i++ {
			_ = manager.Del(&values[i])
		}
	}()
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		value, irrational, matched, e := r.algebraicOperand(h, manager)
		if e != nil || !matched {
			return RewriteFailed, e
		}
		values[i] = value
		constructed++
		if irrational {
			irrationals++
		} else {
			rationals++
		}
	}
	if irrationals == 0 || irrationals == 1 && rationals == 0 {
		return RewriteFailed, nil
	}
	var accumulator polynomial.Algebraic
	if err = manager.Copy(&accumulator, &values[0]); err != nil {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&accumulator) }()
	for i := 1; i < len(values); i++ {
		var next polynomial.Algebraic
		if kind == arithOpAdd {
			err = manager.Add(&accumulator, &values[i], &next)
		} else {
			err = manager.Mul(&accumulator, &values[i], &next)
		}
		if err != nil {
			return RewriteFailed, err
		}
		if err = manager.Del(&accumulator); err != nil {
			_ = manager.Del(&next)
			return RewriteFailed, err
		}
		accumulator = next
	}
	h, err := r.makeAlgebraicNumeral(manager, &accumulator)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, h)
}

func (r *ArithRewriter) rewriteAlgebraicDivision(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if !r.algebraicSimplify {
		return RewriteFailed, nil
	}
	leftSyntax, err := r.algebraicNumeralSyntax(left)
	if err != nil {
		return RewriteFailed, err
	}
	rightSyntax, err := r.algebraicNumeralSyntax(right)
	if err != nil || !leftSyntax && !rightSyntax {
		return RewriteFailed, err
	}
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return RewriteFailed, err
	}
	a, ai, am, err := r.algebraicOperand(left, manager)
	if err != nil || !am {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&a) }()
	b, bi, bm, err := r.algebraicOperand(right, manager)
	if err != nil || !bm {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&b) }()
	if !ai && !bi {
		return RewriteFailed, nil
	}
	zero, err := manager.IsZero(&b)
	if err != nil || zero {
		return RewriteFailed, err
	}
	var quotient polynomial.Algebraic
	if err = manager.Div(&a, &b, &quotient); err != nil {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&quotient) }()
	h, err := r.makeAlgebraicNumeral(manager, &quotient)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, h)
}

func (r *ArithRewriter) rewriteAlgebraicPower(base ast.ASTHandle, exponent *big.Rat, out ast.ASTRef) (RewriteStatus, error) {
	if !r.algebraicSimplify || exponent == nil {
		return RewriteFailed, nil
	}
	syntax, err := r.algebraicNumeralSyntax(base)
	if err != nil {
		return RewriteFailed, err
	}
	_, _, rational, err := r.numeral(base)
	if err != nil || !syntax && !rational {
		return RewriteFailed, err
	}
	numerator := new(big.Int).Set(exponent.Num())
	negative := numerator.Sign() < 0
	if negative {
		numerator.Neg(numerator)
	}
	denominator := exponent.Denom()
	if numerator.Sign() == 0 || !numerator.IsUint64() || !denominator.IsUint64() {
		return RewriteFailed, nil
	}
	n, d := numerator.Uint64(), denominator.Uint64()
	if n > uint64(r.maxDegree) || d > uint64(r.maxDegree) || n > uint64(^uint32(0)) || d > uint64(^uint32(0)) {
		return RewriteFailed, nil
	}
	integer, err := r.integerTerm(base)
	if err != nil || negative && integer {
		return RewriteFailed, err
	}
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return RewriteFailed, err
	}
	value, _, matched, err := r.algebraicOperand(base, manager)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&value) }()
	if d%2 == 0 {
		isNegative, e := manager.IsNeg(&value)
		if e != nil {
			return RewriteFailed, e
		}
		if isNegative && n%2 == 1 {
			return RewriteFailed, nil
		}
	}
	var powered polynomial.Algebraic
	if err = manager.Power(&value, uint32(n), &powered); err != nil {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&powered) }()
	var rooted polynomial.Algebraic
	if err = manager.Root(&powered, uint32(d), &rooted); err != nil {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&rooted) }()
	if negative {
		if err = manager.Inv(&rooted); err != nil {
			return RewriteFailed, err
		}
	}
	h, err := r.makeAlgebraicNumeral(manager, &rooted)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, h)
}
