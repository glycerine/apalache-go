package ast

// Source: src/ast/arith_decl_plugin.cpp:400-449.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// The lexical rational parameter stays at this address if cleanup stops.
type astArithNumeralFrame struct {
	owner                                *astArithmeticPluginOwner
	invocation                           uint64
	ownDone                              bool
	bodyCause                            error
	value                                unumeral.OwnedRational
	copy                                 unumeral.RationalServiceInvocation
	active, placed, complete, sourceDone bool
	sourceCause                          error
}

func (f *astArithNumeralFrame) destroy(m *ASTManager, own bool) error {
	if !f.active {
		return nil
	}
	if !f.sourceDone && f.sourceCause == nil {
		f.sourceCause = f.copy.SourceReady()
		if f.sourceCause == nil && f.placed && f.complete {
			f.sourceCause = m.numerals.Del(&f.value)
			if f.sourceCause == nil {
				f.placed = false
			}
		}
		f.sourceDone = f.sourceCause == nil
	}
	if !own {
		return f.sourceCause
	}
	if e := f.copy.Cleanup(false); e != nil {
		return e
	}
	if f.placed {
		if e := m.numerals.CleanupValue(&f.value); e != nil {
			return e
		}
		f.placed = false
	}
	if e := f.copy.Cleanup(true); e != nil {
		return e
	}
	if e := f.copy.EndContext(); e != nil {
		return e
	}
	f.ownDone = true
	f.active = false
	return nil
}

func (m *ASTManager) mkArithmeticNumeral(value *unumeral.OwnedRational, integer bool, zero bool) (result astNodeRef, err error) {
	return m.mkArithmeticNumeralCaptured(value, integer, zero, nil)
}

func (m *ASTManager) mkArithmeticNumeralCaptured(value *unumeral.OwnedRational, integer bool, zero bool, call *ASTArithmeticNumeralInvocation) (result astNodeRef, err error) {
	p := m.arithmeticPlugin
	if p == nil || !p.published {
		return result, ErrASTHandle
	}
	if p.numeral.active {
		return result, ErrASTPending
	}
	if integer && !zero {
		isInt, e := m.numerals.IsInt(value)
		if e != nil {
			return result, e
		}
		if !isInt {
			return result, ASTException{z3exception.NewDefaultException("invalid rational value passed as an integer")}
		}
	}
	u, cached := uint32(0), zero
	if !zero {
		unsigned, e := m.numerals.IsUnsigned(value)
		if e != nil {
			return result, e
		}
		if unsigned {
			u, e = m.numerals.Unsigned(value)
			if e != nil {
				return result, e
			}
			cached = u < 16
		}
	}
	which, sortIndex, flag := 1, 0, int32(0)
	if integer && !p.convertIntNumerals {
		which, sortIndex = 0, 1
	}
	if integer && (!cached || !p.convertIntNumerals) {
		flag = 1
	}
	if cached && uint64(u) < uint64(len(p.small[which])) {
		r := p.small[which][u]
		if r.slot != 0 {
			return r, nil
		}
	}
	if p.numeralSerial == math.MaxUint64 {
		return result, arena.ErrBounds
	}
	p.numeralSerial++
	f := &p.numeral
	*f = astArithNumeralFrame{owner: p, invocation: p.numeralSerial, active: true}
	if call != nil {
		call.frame, call.invocation = f, f.invocation
	}
	defer func() {
		f.bodyCause = err
		source := f.destroy(m, false)
		err = errors.Join(err, source)
		if source == nil {
			err = errors.Join(err, f.destroy(m, true))
		}
		if call != nil {
			call.bodyCause, call.sourceCause = f.bodyCause, f.sourceCause
			call.sourceDone, call.retired = f.sourceDone, f.ownDone
		}
	}()
	f.value, err = m.numerals.NewValue()
	if err != nil {
		return result, err
	}
	f.placed = true
	if !zero {
		if err = m.numerals.CopyWithInvocation(&f.value, value, &f.copy); err != nil {
			return result, err
		}
	}
	f.complete = true
	parameter := astParameterRecord{Kind: ASTParameterRational, Rational: f.value}
	if err = m.beginFormatInfo(false, p.family, 0, &parameter); err != nil {
		return result, err
	}
	defer m.endFormatInfo()
	m.formatLexical.secondParameter = astParameterRecord{Kind: ASTParameterInt, integer: flag}
	m.formatLexical.localPair = true
	m.formatLexical.info.count = 2
	epoch := m.formatLexical.epoch * 2
	ps := [2]ASTParameter{{m.origin, math.MaxUint32 - 1, epoch}, {m.origin, math.MaxUint32 - 1, epoch + 1}}
	nameIndex := 1
	if sortIndex == 1 {
		nameIndex = 0
	}
	decl, e := m.pluginDecl(p.names[nameIndex], p.family, 0, ps[:], nil, p.fields[sortIndex], ASTFuncFlags{}, false)
	if e != nil {
		return result, e
	}
	result, err = m.mkApp(decl, nil)
	if err != nil || !cached {
		return result, err
	}
	if err = m.incNode(result); err != nil {
		return result, err
	}
	v := &p.small[which]
	if int(u) >= len(*v) {
		*v = append(*v, make([]astNodeRef, int(u)+1-len(*v))...)
	}
	(*v)[u] = result
	return result, nil
}

func (m *ASTManager) ArithmeticNumeral(value *unumeral.OwnedRational, integer bool) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	r, e := m.mkArithmeticNumeral(value, integer, false)
	return m.external(r), e
}

func (m *ASTManager) arithmeticNumeralDecl(ps []ASTParameter, arity int) (astNodeRef, error) {
	if len(ps) != 2 || arity != 0 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid numeral declaration")}
	}
	v, e := m.parameter(ps[0])
	if e != nil {
		return astNodeRef{}, e
	}
	flag, e := m.parameter(ps[1])
	if e != nil {
		return astNodeRef{}, e
	}
	if v.Kind != ASTParameterRational || flag.Kind != ASTParameterInt {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid numeral declaration")}
	}
	p := m.arithmeticPlugin
	name, sort := p.names[1], p.fields[0]
	if flag.integer != 0 {
		name, sort = p.names[0], p.fields[1]
	}
	return m.pluginDecl(name, p.family, 0, ps, nil, sort, ASTFuncFlags{}, false)
}
