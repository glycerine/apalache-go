package ast

// Source: src/ast/pb_decl_plugin.cpp:103-181,200-217.

import (
	"errors"
	"math"
	"math/big"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) pseudoBooleanRat(value *unumeral.OwnedRational) (*big.Rat, error) {
	text, e := m.numerals.String(value)
	if e != nil {
		return nil, e
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ErrASTDomain
	}
	return r, nil
}

func pbLCM(a, b *big.Int) *big.Int {
	if a.Sign() == 0 || b.Sign() == 0 {
		return new(big.Int)
	}
	g := new(big.Int).GCD(nil, nil, a, b)
	r := new(big.Int).Quo(new(big.Int).Set(a), g)
	r.Mul(r, b)
	return r.Abs(r)
}

func pbFloor(r *big.Rat) *big.Int {
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if r.Sign() < 0 && rem.Sign() != 0 {
		q.Sub(q, big.NewInt(1))
	}
	return q
}

func pbCeil(r *big.Rat) *big.Int {
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if r.Sign() > 0 && rem.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

func (m *ASTManager) pseudoBooleanIntegerParameter(value *big.Int) (ASTParameter, error) {
	if value.IsInt64() {
		n := value.Int64()
		if n >= math.MinInt32 && n <= math.MaxInt32 {
			return m.newParameterInt(int32(n))
		}
	}
	r, e := m.numerals.NewValue()
	if e != nil {
		return ASTParameter{}, e
	}
	if e = m.numerals.SetString(&r, value.String()); e != nil {
		_ = m.numerals.Del(&r)
		return ASTParameter{}, e
	}
	p, e := m.newParameterRational(&r)
	return p, errors.Join(e, m.numerals.Del(&r))
}

func (m *ASTManager) pseudoBooleanBuild(kind int32, coefficients []*big.Int, bound *big.Int, args []ASTHandle) (result ASTHandle, err error) {
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	if p == nil || len(coefficients) != len(args) {
		return result, ErrASTDomain
	}
	parameters := make([]ASTParameter, 0, len(args)+1)
	defer func() {
		for _, value := range parameters {
			err = errors.Join(err, m.dropParameter(value))
		}
	}()
	parameter, e := m.pseudoBooleanIntegerParameter(bound)
	if e != nil {
		return result, e
	}
	parameters = append(parameters, parameter)
	for _, coefficient := range coefficients {
		parameter, e = m.pseudoBooleanIntegerParameter(coefficient)
		if e != nil {
			return result, e
		}
		parameters = append(parameters, parameter)
	}
	r, e := m.mkFamilyApp(p.family, kind, parameters, args)
	return m.external(r), e
}

func (m *ASTManager) pseudoBooleanNormalize(coefficients []unumeral.OwnedRational, bound *unumeral.OwnedRational) ([]*big.Int, *big.Rat, error) {
	if bound == nil {
		return nil, nil, ErrASTHandle
	}
	values := make([]*big.Rat, len(coefficients))
	lcm := big.NewInt(1)
	for i := range coefficients {
		value, e := m.pseudoBooleanRat(&coefficients[i])
		if e != nil {
			return nil, nil, e
		}
		values[i] = value
		lcm = pbLCM(lcm, value.Denom())
	}
	k, e := m.pseudoBooleanRat(bound)
	if e != nil {
		return nil, nil, e
	}
	result := make([]*big.Int, len(values))
	for i, value := range values {
		scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(lcm))
		if !scaled.IsInt() {
			return nil, nil, ErrASTDomain
		}
		result[i] = new(big.Int).Set(scaled.Num())
	}
	k.Mul(k, new(big.Rat).SetInt(lcm))
	return result, k, nil
}

func (m *ASTManager) pseudoBooleanRelation(kind int32, coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if len(coefficients) != len(args) {
		return ASTHandle{}, ErrASTDomain
	}
	values, k, e := m.pseudoBooleanNormalize(coefficients, bound)
	if e != nil {
		return ASTHandle{}, e
	}
	var integer *big.Int
	switch kind {
	case ASTPBOpLE:
		integer = pbFloor(k)
	case ASTPBOpGE:
		integer = pbCeil(k)
	case ASTPBOpEQ:
		if !k.IsInt() {
			r, e := m.mkApp(m.basic.declarations[ASTOpFalse], nil)
			return m.external(r), e
		}
		integer = new(big.Int).Set(k.Num())
	default:
		return ASTHandle{}, ErrASTDomain
	}
	return m.pseudoBooleanBuild(kind, values, integer, args)
}

func (m *ASTManager) MkPseudoBooleanLE(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (ASTHandle, error) {
	return m.pseudoBooleanRelation(ASTPBOpLE, coefficients, args, bound)
}
func (m *ASTManager) MkPseudoBooleanGE(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (ASTHandle, error) {
	return m.pseudoBooleanRelation(ASTPBOpGE, coefficients, args, bound)
}
func (m *ASTManager) MkPseudoBooleanEQ(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (ASTHandle, error) {
	return m.pseudoBooleanRelation(ASTPBOpEQ, coefficients, args, bound)
}

func (m *ASTManager) pseudoBooleanCardinality(kind int32, args []ASTHandle, k uint32) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	if p == nil {
		return result, ErrASTHandle
	}
	parameter, e := m.newParameterInt(int32(k))
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(parameter)) }()
	r, e := m.mkFamilyApp(p.family, kind, []ASTParameter{parameter}, args)
	return m.external(r), e
}

func (m *ASTManager) MkPseudoBooleanAtMost(args []ASTHandle, k uint32) (ASTHandle, error) {
	return m.pseudoBooleanCardinality(ASTPBOpAtMost, args, k)
}
func (m *ASTManager) MkPseudoBooleanAtLeast(args []ASTHandle, k uint32) (ASTHandle, error) {
	return m.pseudoBooleanCardinality(ASTPBOpAtLeast, args, k)
}

// MkPseudoBooleanLT preserves the source complement transformation:
// a*x < k becomes a*(not x) >= -floor(k) + sum(a) + 1 after normalization.
func (m *ASTManager) MkPseudoBooleanLT(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	if len(coefficients) != len(args) {
		return result, ErrASTDomain
	}
	values, k, e := m.pseudoBooleanNormalize(coefficients, bound)
	if e != nil {
		return result, e
	}
	transformed := new(big.Int).Neg(pbFloor(k))
	transformed.Add(transformed, big.NewInt(1))
	negated := make([]ASTHandle, len(args))
	for i, arg := range args {
		transformed.Add(transformed, values[i])
		n, e := m.mkFamilyApp(0, ASTOpNot, nil, []ASTHandle{arg})
		if e != nil {
			return result, e
		}
		negated[i] = m.external(n)
	}
	return m.pseudoBooleanBuild(ASTPBOpGE, values, transformed, negated)
}
