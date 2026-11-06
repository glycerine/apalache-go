package arith

// Source: src/tactic/arith/fm_tactic.cpp:1284-1358.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func fmCoefficient(c *FMConstraint, x uint32) (*big.Rat, bool) {
	for i, v := range c.Variables {
		if v == x {
			return &c.Coefficients[i], true
		}
	}
	return nil, false
}

func fmResolvePolynomial(variables *FMVariableTable, lower, upper *FMConstraint, x uint32) (xs []uint32, coefficients []big.Rat, constant big.Rat, strict bool, err error) {
	la, lok := fmCoefficient(lower, x)
	ub, uok := fmCoefficient(upper, x)
	if variables == nil || !lok || !uok || la.Sign() >= 0 || ub.Sign() <= 0 {
		return nil, nil, big.Rat{}, false, ast.ErrASTDomain
	}
	a := new(big.Rat).Neg(la)
	b := new(big.Rat).Set(ub)
	if ast.ASTSourceDebug && variables.IsInteger(x) && !fmIsOne(a) && !fmIsOne(b) {
		return nil, nil, big.Rat{}, false, ast.ErrASTDomain
	}
	constant.Mul(&lower.Constant, b)
	constant.Add(&constant, new(big.Rat).Mul(&upper.Constant, a))
	strict = lower.Strict || upper.Strict
	positions := make(map[uint32]int, len(lower.Variables)+len(upper.Variables))
	for i, v := range lower.Variables {
		if v == x {
			continue
		}
		positions[v] = len(xs)
		xs = append(xs, v)
		coefficients = append(coefficients, *new(big.Rat).Mul(&lower.Coefficients[i], b))
	}
	for i, v := range upper.Variables {
		if v == x {
			continue
		}
		term := new(big.Rat).Mul(&upper.Coefficients[i], a)
		if p, found := positions[v]; found {
			coefficients[p].Add(&coefficients[p], term)
		} else {
			positions[v] = len(xs)
			xs = append(xs, v)
			coefficients = append(coefficients, *term)
		}
	}
	write := 0
	allInteger := true
	for i := range xs {
		if coefficients[i].Sign() == 0 {
			continue
		}
		if !variables.IsInteger(xs[i]) {
			allInteger = false
		}
		xs[write] = xs[i]
		coefficients[write].Set(&coefficients[i])
		write++
	}
	xs, coefficients = xs[:write], coefficients[:write]
	if allInteger && strict {
		strict = false
		constant.Sub(&constant, big.NewRat(1, 1))
	}
	return xs, coefficients, constant, strict, nil
}
