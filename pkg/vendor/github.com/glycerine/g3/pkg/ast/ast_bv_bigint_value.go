package ast

import (
	"math/big"
)

// BitVectorNumeralBigInt reads a bit-vector numeral exactly without exporting
// an owning rational temporary. A false match is returned for other ASTs.
func (m *ASTManager) BitVectorNumeralBigInt(h ASTHandle) (value *big.Int, width uint32, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		err = e
		return
	}
	matched, err = m.isBitVectorNumeral(r)
	if err != nil || !matched {
		return
	}
	info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if e != nil {
		err = e
		return
	}
	token, e := m.infoParameter(info, 0)
	if e != nil {
		err = e
		return
	}
	parameter, e := m.parameter(token)
	if e != nil {
		err = e
		return
	}
	if parameter.Kind != ASTParameterRational {
		err = ErrASTDomain
		return
	}
	denominatorIsOne, e := m.numerals.Manager.Z.IsOne(&parameter.Rational.Raw.Den)
	if e != nil {
		err = e
		return
	}
	if !denominatorIsOne {
		err = ErrASTDomain
		return
	}
	text, e := m.numerals.Manager.Z.String(&parameter.Rational.Raw.Num)
	if e != nil {
		err = e
		return
	}
	value, matched = new(big.Int).SetString(text, 10)
	if !matched {
		err = ErrASTDomain
		return
	}
	token, e = m.infoParameter(info, 1)
	if e != nil {
		err = e
		return
	}
	parameter, e = m.parameter(token)
	if e != nil {
		err = e
		return
	}
	if parameter.Kind != ASTParameterInt || parameter.integer < 0 {
		err = ErrASTDomain
		return
	}
	width = uint32(parameter.integer)
	return
}

// BitVectorConcatArguments exposes the concrete operand list for a BV concat.
func (m *ASTManager) BitVectorConcatArguments(h ASTHandle) (arguments []ASTHandle, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		err = e
		return
	}
	matched, err = m.bitVectorAppKind(r, 37)
	if err != nil || !matched {
		return
	}
	app := m.appAt(r.slot)
	arguments = make([]ASTHandle, app.count)
	for i := uint32(0); i < app.count; i++ {
		child, readErr := m.readInternal(app.args, i)
		if readErr != nil {
			err = readErr
			arguments = nil
			return
		}
		arguments[i] = m.external(child)
	}
	return
}
