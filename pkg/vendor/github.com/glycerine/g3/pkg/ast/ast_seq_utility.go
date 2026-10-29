package ast

// Source: src/ast/seq_decl_plugin.cpp:959-967,993-1018;
// src/ast/seq_decl_plugin.h:277-306,319-335.
// Reached consumer: src/ast/rewriter/seq_rewriter.cpp:466-481.

import "errors"

func (m *ASTManager) sequenceBinaryParts(r astNodeRef, kind int32) (astNodeRef, astNodeRef, bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return astNodeRef{}, astNodeRef{}, false, nil
	}
	a := m.appAt(r.slot)
	family, actualKind, err := m.declIdentity(a.decl)
	if err != nil || family != m.sequencePlugin.family || actualKind != kind || a.count != 2 {
		return astNodeRef{}, astNodeRef{}, false, err
	}
	left, err := m.readInternal(a.args, 0)
	if err != nil {
		return astNodeRef{}, astNodeRef{}, false, err
	}
	right, err := m.readInternal(a.args, 1)
	return left, right, err == nil, err
}

func (m *ASTManager) SequenceConcatParts(h ASTHandle) (left, right ASTHandle, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	l, rr, matched, err := m.sequenceBinaryParts(r, ASTSequenceOpConcat)
	return m.external(l), m.external(rr), matched, err
}

func (m *ASTManager) sequenceFlattenConcat(r astNodeRef, out *[]ASTHandle) error {
	for {
		left, right, matched, err := m.sequenceBinaryParts(r, ASTSequenceOpConcat)
		if err != nil {
			return err
		}
		if !matched {
			break
		}
		if err = m.sequenceFlattenConcat(left, out); err != nil {
			return err
		}
		r = right
	}
	empty, err := m.isSequenceEmptyRef(r)
	if err != nil {
		return err
	}
	if !empty {
		*out = append(*out, m.external(r))
	}
	return nil
}

// SequenceFlattenConcat returns source-order concat leaves and omits sequence
// empty values, matching seq_util::str::get_concat.
func (m *ASTManager) SequenceFlattenConcat(h ASTHandle) ([]ASTHandle, error) {
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return nil, err
	}
	result := make([]ASTHandle, 0, 4)
	if err = m.sequenceFlattenConcat(r, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *ASTManager) sequenceConcatEdge(r astNodeRef, left bool) (astNodeRef, error) {
	for {
		first, second, matched, err := m.sequenceBinaryParts(r, ASTSequenceOpConcat)
		if err != nil || !matched {
			return r, err
		}
		if left {
			r = first
		} else {
			r = second
		}
	}
}

func (m *ASTManager) SequenceLeftmostConcat(h ASTHandle) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, err
	}
	r, err = m.sequenceConcatEdge(r, true)
	return m.external(r), err
}

func (m *ASTManager) SequenceRightmostConcat(h ASTHandle) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, err
	}
	r, err = m.sequenceConcatEdge(r, false)
	return m.external(r), err
}

// RegexLoopParameters recognizes the unary declaration-parameter forms of
// re.loop. bounded is true for the two-parameter lower/upper form.
func (m *ASTManager) RegexLoopParameters(h ASTHandle) (body ASTHandle, lower, upper int32, bounded, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, 0, 0, false, false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return ASTHandle{}, 0, 0, false, false, nil
	}
	a := m.appAt(r.slot)
	family, kind, err := m.declIdentity(a.decl)
	if err != nil || family != m.sequencePlugin.family || kind != 20 || a.count != 1 {
		return ASTHandle{}, 0, 0, false, false, err
	}
	info, err := m.funcInfo(m.declAt(a.decl.slot).info)
	if err != nil || info.count < 1 || info.count > 2 {
		return ASTHandle{}, 0, 0, false, false, err
	}
	values := [2]int32{}
	for i := uint32(0); i < info.count; i++ {
		token, e := m.infoParameter(info, i)
		if e != nil {
			return ASTHandle{}, 0, 0, false, false, e
		}
		parameter, e := m.parameter(token)
		if e != nil || parameter.Kind != ASTParameterInt {
			if e == nil {
				e = ErrASTDomain
			}
			return ASTHandle{}, 0, 0, false, false, e
		}
		values[i] = parameter.integer
	}
	child, err := m.readInternal(a.args, 0)
	if err != nil {
		return ASTHandle{}, 0, 0, false, false, err
	}
	return m.external(child), values[0], values[1], info.count == 2, true, nil
}

// SequenceUnitByteElementValue recognizes the reached (_ BitVec 8) numeral
// argument profile used by the seq.unit rewriter.
func (m *ASTManager) SequenceUnitByteElementValue(h ASTHandle) (value byte, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, err := m.requireExpr(h)
	if err != nil {
		return 0, false, err
	}
	numeral, err := m.numerals.NewValue()
	if err != nil {
		return 0, false, err
	}
	defer func() { err = errors.Join(err, m.numerals.Del(&numeral)) }()
	var width uint32
	matched, err = m.readBitVectorNumeral(r, &numeral, &width)
	if err != nil || !matched || width != 8 {
		return 0, false, err
	}
	u, err := m.numerals.Uint64(&numeral)
	if err != nil || u > 255 {
		if err == nil {
			err = ErrASTDomain
		}
		return 0, false, err
	}
	return byte(u), true, nil
}

func (m *ASTManager) SequenceIntToStringArgument(h ASTHandle) (ASTHandle, bool, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return ASTHandle{}, false, nil
	}
	a := m.appAt(r.slot)
	family, kind, err := m.declIdentity(a.decl)
	if err != nil || family != m.sequencePlugin.family || kind != 27 || a.count != 1 {
		return ASTHandle{}, false, err
	}
	argument, err := m.readInternal(a.args, 0)
	return m.external(argument), err == nil, err
}

func (m *ASTManager) IsRegexEpsilon(h ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return false, nil
	}
	a := m.appAt(r.slot)
	family, kind, err := m.declIdentity(a.decl)
	if err != nil || family != m.sequencePlugin.family || kind != ASTSequenceOpToRegex || a.count != 1 {
		return false, err
	}
	argument, err := m.readInternal(a.args, 0)
	if err != nil {
		return false, err
	}
	return m.isSequenceEmptyRef(argument)
}

func (m *ASTManager) SequenceUnaryOperationArgument(h ASTHandle, kind int32) (ASTHandle, bool, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return ASTHandle{}, false, nil
	}
	a := m.appAt(r.slot)
	family, actualKind, err := m.declIdentity(a.decl)
	if err != nil || family != m.sequencePlugin.family || actualKind != kind || a.count != 1 {
		return ASTHandle{}, false, err
	}
	argument, err := m.readInternal(a.args, 0)
	return m.external(argument), err == nil, err
}

func (m *ASTManager) SequenceUnitArgument(h ASTHandle) (ASTHandle, bool, error) {
	return m.SequenceUnaryOperationArgument(h, ASTSequenceOpUnit)
}

func (m *ASTManager) SequenceBinaryOperationArguments(h ASTHandle, kind int32) (ASTHandle, ASTHandle, bool, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	left, right, matched, err := m.sequenceBinaryParts(r, kind)
	return m.external(left), m.external(right), matched, err
}

// SequenceExtractArguments recognizes the retained three-argument seq.extract
// application used by the sequence contains rewriter.
func (m *ASTManager) SequenceExtractArguments(h ASTHandle) (sequence, offset, length ASTHandle, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, ASTHandle{}, false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return ASTHandle{}, ASTHandle{}, ASTHandle{}, false, nil
	}
	a := m.appAt(r.slot)
	family, kind, err := m.declIdentity(a.decl)
	if err != nil || family != m.sequencePlugin.family || kind != 6 || a.count != 3 {
		return ASTHandle{}, ASTHandle{}, ASTHandle{}, false, err
	}
	x, err := m.readInternal(a.args, 0)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, ASTHandle{}, false, err
	}
	y, err := m.readInternal(a.args, 1)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, ASTHandle{}, false, err
	}
	z, err := m.readInternal(a.args, 2)
	return m.external(x), m.external(y), m.external(z), err == nil, err
}
