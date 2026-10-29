package ast

// Source ast.cpp same_patterns/same_no_patterns and six update overloads.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

const (
	astUpdateKind uint8 = 1 << iota
	astUpdateWeight
	astUpdatePatterns
	astUpdateNoPatterns
)

func (m *ASTManager) sameQuantifierList(p arena.SmallBytesRef, n uint32, values AppArguments) (bool, error) {
	if values.size() != uint64(n) {
		return false, nil
	}
	for i := uint32(0); i < uint32(values.size()); i++ {
		h, e := values.at(i)
		if e != nil {
			return false, e
		}
		r, e := m.readInternal(p, uint32(i))
		if e != nil {
			return false, e
		}
		if m.external(r) != h {
			return false, nil
		}
	}
	return true, nil
}

// Copy the actual retained payload directly into the candidate. The source
// passes these existing arrays to its constructor; no temporary handle slice
// or separately allocated per-element owner is necessary here.
func (m *ASTManager) copyQuantifierPayload(p arena.SmallBytesRef, count, stride uint32) (arena.SmallBytesRef, error) {
	if count == 0 {
		return arena.SmallBytesRef{}, nil
	}
	bytes, e := astPayloadBytes(count, stride)
	if e != nil {
		return arena.SmallBytesRef{}, e
	}
	out, e := m.allocator.Allocate(bytes)
	if e != nil {
		return arena.SmallBytesRef{}, e
	}
	for i := uint32(0); i < bytes; i++ {
		b, e := m.allocator.Read(p, i)
		if e == nil {
			e = m.allocator.Write(out, i, b)
		}
		if e != nil {
			m.allocator.Deallocate(bytes, out)
			return arena.SmallBytesRef{}, e
		}
	}
	return out, nil
}

func (m *ASTManager) checkUpdatePatterns(values AppArguments) error {
	if values.size() > math.MaxUint32 {
		return arena.ErrBounds
	}
	for i := uint32(0); i < uint32(values.size()); i++ {
		h, e := values.at(i)
		if e != nil {
			return e
		}
		r, e := m.requireExpr(h)
		if e != nil {
			return e
		}
		if !ASTSourceDebug {
			continue
		}
		if m.nodeAt(r.slot).kind != ASTApp {
			return ErrASTDomain
		}
		a := m.appAt(r.slot)
		fid, k, e := m.declIdentity(a.decl)
		if e != nil {
			return e
		}
		if fid != 2 || k != 0 {
			return ErrASTDomain
		}
		for i := uint32(0); i < a.count; i++ {
			child, e := m.readInternal(a.args, i)
			if e != nil {
				return e
			}
			if m.nodeAt(child.slot).kind != ASTApp {
				return ErrASTDomain
			}
		}
	}
	return nil
}

func (m *ASTManager) updateQuantifier(h, body ASTHandle, forall bool, weight int32, patterns, noPatterns []ASTHandle, flags uint8) (ASTHandle, error) {
	return m.updateQuantifierArguments(h, body, forall, weight, AppArguments{slice: patterns, count: uint32(len(patterns))}, AppArguments{slice: noPatterns, count: uint32(len(noPatterns))}, flags)
}

func (m *ASTManager) updateQuantifierArguments(h, body ASTHandle, forall bool, weight int32, patterns, noPatterns AppArguments, flags uint8) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	q, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return ASTHandle{}, e
	}
	old := *m.quantifierAt(q.slot)
	if flags&astUpdateWeight != 0 {
		if old.weight == weight {
			return h, nil
		}
		m.traceQuantifierWeight(m.nodeAt(q.slot).id, old.weight, weight)
		body = m.external(old.body)
	} else {
		if flags&astUpdateKind == 0 {
			forall = old.forall
		}
		same := body == m.external(old.body) && forall == old.forall
		if same && flags&astUpdatePatterns != 0 {
			same, e = m.sameQuantifierList(old.patterns, old.patternCount, patterns)
			if e != nil {
				return ASTHandle{}, e
			}
		}
		if same && flags&astUpdateNoPatterns != 0 {
			same, e = m.sameQuantifierList(old.noPatterns, old.noPatternCount, noPatterns)
			if e != nil {
				return ASTHandle{}, e
			}
		}
		if same {
			return h, nil
		}
	}
	next := old
	next.nextFree = 0
	if flags&astUpdateKind != 0 {
		next.forall = forall
	}
	if flags&astUpdateWeight != 0 {
		next.weight = weight
	}
	if flags&astUpdatePatterns != 0 {
		if patterns.size() > math.MaxUint32 {
			return ASTHandle{}, arena.ErrBounds
		}
		next.patternCount = uint32(patterns.size())
		if flags&astUpdateNoPatterns == 0 && next.patternCount != 0 {
			next.noPatternCount = 0
		}
	}
	if flags&astUpdateNoPatterns != 0 {
		if noPatterns.size() > math.MaxUint32 {
			return ASTHandle{}, arena.ErrBounds
		}
		next.noPatternCount = uint32(noPatterns.size())
	}
	if next.patternCount != 0 && next.noPatternCount != 0 || ASTSourceDebug && next.count == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	next.body, e = m.requireExpr(body)
	if e != nil {
		return ASTHandle{}, e
	}
	if flags&astUpdatePatterns != 0 {
		if e = m.checkUpdatePatterns(patterns); e != nil {
			return ASTHandle{}, e
		}
	}
	if flags&astUpdateNoPatterns != 0 {
		for i := uint32(0); i < noPatterns.count; i++ {
			h, e := noPatterns.at(i)
			if e != nil {
				return ASTHandle{}, e
			}
			if _, e = m.requireExpr(h); e != nil {
				return ASTHandle{}, e
			}
		}
	}
	next.depth = m.depth(next.body) + 1
	next.unusedVars = true
	next.labels = m.hasLabels(next.body)
	// Publish only payloads actually allocated by this candidate. Early cleanup
	// must never free the original quantifier's borrowed arrays.
	next.sorts = arena.SmallBytesRef{}
	next.names = arena.SmallBytesRef{}
	next.patterns = arena.SmallBytesRef{}
	next.noPatterns = arena.SmallBytesRef{}
	r, e := m.allocateNode(ASTQuantifier)
	if e != nil {
		return ASTHandle{}, e
	}
	*m.quantifierAt(r.slot) = next
	v := m.quantifierAt(r.slot)
	v.sorts, e = m.copyQuantifierPayload(old.sorts, old.count, astRefBytes)
	if e == nil {
		v.names, e = m.copyQuantifierPayload(old.names, old.count, astSymbolBytes)
	}
	if e == nil {
		if flags&astUpdatePatterns != 0 {
			v.patterns, e = m.newASTPayloadArguments(patterns)
		} else {
			v.patterns, e = m.copyQuantifierPayload(old.patterns, old.patternCount, astRefBytes)
		}
	}
	if e == nil {
		if flags&astUpdateNoPatterns != 0 {
			v.noPatterns, e = m.newASTPayloadArguments(noPatterns)
		} else {
			v.noPatterns, e = m.copyQuantifierPayload(old.noPatterns, next.noPatternCount, astRefBytes)
		}
	}
	if e != nil {
		m.freeNodeStorage(r)
		return ASTHandle{}, e
	}
	r, e = m.registerNode(r)
	return m.external(r), e
}

func (m *ASTManager) UpdateQuantifierBody(q, body ASTHandle) (ASTHandle, error) {
	return m.updateQuantifier(q, body, false, 0, nil, nil, 0)
}
func (m *ASTManager) UpdateQuantifierWeight(q ASTHandle, weight int32) (ASTHandle, error) {
	return m.updateQuantifier(q, ASTHandle{}, false, weight, nil, nil, astUpdateWeight)
}
func (m *ASTManager) UpdateQuantifierKind(q ASTHandle, forall bool, body ASTHandle) (ASTHandle, error) {
	return m.updateQuantifier(q, body, forall, 0, nil, nil, astUpdateKind)
}
func (m *ASTManager) UpdateQuantifierPatterns(q ASTHandle, patterns []ASTHandle, body ASTHandle) (ASTHandle, error) {
	return m.updateQuantifier(q, body, false, 0, patterns, nil, astUpdatePatterns)
}
func (m *ASTManager) UpdateQuantifierAnnotations(q ASTHandle, patterns, noPatterns []ASTHandle, body ASTHandle) (ASTHandle, error) {
	return m.updateQuantifier(q, body, false, 0, patterns, noPatterns, astUpdatePatterns|astUpdateNoPatterns)
}
func (m *ASTManager) UpdateQuantifierKindPatterns(q ASTHandle, forall bool, patterns []ASTHandle, body ASTHandle) (ASTHandle, error) {
	return m.updateQuantifier(q, body, forall, 0, patterns, nil, astUpdateKind|astUpdatePatterns)
}
