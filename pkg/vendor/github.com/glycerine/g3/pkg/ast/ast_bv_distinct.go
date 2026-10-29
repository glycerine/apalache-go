package ast

// Source: src/ast/bv_decl_plugin.cpp:636-669; src/util/rational.h:131-139.

import "errors"

// The two lexical offsets and full-expression power/mod temporaries are owned
// by the existing address-stable numeric continuation. No numeral-cache
// normalization is assumed by this predicate: raw declaration parameters also
// reach the original copy, power, mod and assignment operations.
func (m *ASTManager) bitVectorOffsetTerm(fid FamilyID, a astNodeRef, f *astBVNumeralFrame, side int) (astNodeRef, error) {
	app := m.appAt(a.slot)
	family, kind, e := m.declIdentity(app.decl)
	if e != nil {
		return astNodeRef{}, e
	}
	child := side * 4
	q := m.numerals
	if app.count != 2 || family != fid || kind != 4 {
		return a, q.SetInt64WithInvocation(&f.values[side], 0, &f.children[child])
	}
	offset, e := m.readInternal(app.args, 0)
	if e != nil {
		return astNodeRef{}, e
	}
	if m.nodeAt(offset.slot).kind != ASTApp {
		return a, q.SetInt64WithInvocation(&f.values[side], 0, &f.children[child])
	}
	decl := m.appAt(offset.slot).decl
	family, kind, e = m.declIdentity(decl)
	if e != nil {
		return astNodeRef{}, e
	}
	if family != fid || kind != 0 {
		return a, q.SetInt64WithInvocation(&f.values[side], 0, &f.children[child])
	}
	info, e := m.funcInfo(m.declAt(decl.slot).info)
	if e != nil {
		return astNodeRef{}, e
	}
	param, e := m.infoParameter(info, 0)
	if e != nil {
		return astNodeRef{}, e
	}
	value, e := m.parameter(param)
	if e != nil {
		return astNodeRef{}, e
	}
	if value.Kind != ASTParameterRational {
		return astNodeRef{}, ErrASTDomain
	}
	if e = q.CopyWithInvocation(&f.values[side], &value.Rational, &f.children[child]); e != nil {
		return astNodeRef{}, e
	}
	param, e = m.infoParameter(info, 1)
	if e != nil {
		return astNodeRef{}, e
	}
	width, e := m.parameter(param)
	if e != nil {
		return astNodeRef{}, e
	}
	if width.Kind != ASTParameterInt {
		return astNodeRef{}, ErrASTDomain
	}
	term, e := m.readInternal(app.args, 1)
	if e != nil {
		return astNodeRef{}, e
	}
	powerSlot := 2 + side*2
	modSlot := powerSlot + 1
	power, e := q.PowerOfTwoWithInvocation(uint32(width.integer), &f.children[child+1])
	if e != nil {
		return astNodeRef{}, e
	}
	f.values[powerSlot] = power
	f.placed[powerSlot], f.complete[powerSlot] = true, true
	reduced, e := q.ModWithInvocation(&f.values[side], &f.values[powerSlot], &f.children[child+2])
	if e != nil {
		return astNodeRef{}, e
	}
	f.values[modSlot] = reduced
	f.placed[modSlot], f.complete[modSlot] = true, true
	if e = q.CopyWithInvocation(&f.values[side], &f.values[modSlot], &f.children[child+3]); e != nil {
		return astNodeRef{}, e
	}
	// Full-expression temporaries end before the second offset is extracted.
	for i := child + 3; i >= child+1; i-- {
		if e = f.children[i].SourceReady(); e != nil {
			return astNodeRef{}, e
		}
	}
	if e = f.destroyOne(modSlot, false); e != nil {
		return astNodeRef{}, e
	}
	if e = f.destroyOne(powerSlot, false); e != nil {
		return astNodeRef{}, e
	}
	return term, nil
}

func (m *ASTManager) bitVectorOffsetDistinct(fid FamilyID, a, b astNodeRef) (distinct bool, result error) {
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return false, e
	}
	f.offsetDistinct = true
	defer func() {
		f.bodyCause = result
		result = errors.Join(result, f.destroy(false))
		if result == nil && f.sourceDone {
			result = f.destroy(true)
			if result == nil {
				for _, i := range f.childOrder() {
					if i >= 0 {
						if e := f.children[i].EndContext(); e != nil {
							result = e
							return
						}
					}
				}
				f.owner.numeralFrames = f.next
				m.BudgetField.Release(f.header)
				f.header = 0
				f.retired = true
			}
		}
	}()
	for i := 0; i < 2; i++ {
		f.values[i], e = m.numerals.NewValue()
		if e != nil {
			return false, e
		}
		f.placed[i], f.complete[i] = true, true
	}
	aTerm, e := m.bitVectorOffsetTerm(fid, a, f, 0)
	if e != nil {
		return false, e
	}
	bTerm, e := m.bitVectorOffsetTerm(fid, b, f, 1)
	if e != nil {
		return false, e
	}
	if aTerm == bTerm {
		different, e := m.numerals.NotEqual(&f.values[0], &f.values[1])
		if e != nil || different {
			return different, e
		}
	}
	// Only after the offset lemma does the source call decl_plugin::are_distinct.
	if a == b {
		return false, nil
	}
	first, e := m.pluginUniqueValue(fid, a)
	if e != nil || !first {
		return false, e
	}
	return m.pluginUniqueValue(fid, b)
}
