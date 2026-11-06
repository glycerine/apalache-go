package ast

import (
	"github.com/glycerine/g3/pkg/arena"
)

// MatchProofHyperResolve appends owning premise handles to the caller slice;
// the caller must DecRef each appended handle. Substitution references belong
// to out, while conclusion uses its separately supplied owning ASTRef.
func (m *ASTManager) MatchProofHyperResolve(p ASTHandle, premises *[]ASTHandle, conclusion ASTRef, out *ASTProofHyperOutput) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	yes, e := m.proofKind(p, ASTPrHyperResolve)
	if e != nil || !yes {
		return false, e
	}
	r, e := m.requireKind(p, ASTApp)
	if e != nil {
		return false, e
	}
	a := m.appAt(r.slot)
	if ASTSourceDebug && a.count == 0 {
		return false, ErrASTDomain
	}
	for i := uint32(0); i+1 < a.count; i++ {
		h, e := m.proofArg(p, i)
		if e != nil {
			return false, e
		}
		child, e := m.requireKind(h, ASTApp)
		if e != nil {
			return false, e
		} // source to_app
		if premises == nil {
			return false, ErrASTHandle
		}
		if e = m.incNode(child); e != nil {
			return false, e
		}
		*premises = append(*premises, h)
	}
	if a.count == 0 {
		return false, arena.ErrBounds
	}
	f, e := m.proofArg(p, a.count-1)
	if e != nil {
		return false, e
	}
	child, e := m.requireExpr(f)
	if e != nil {
		return false, e
	}
	if e = m.assignRef(conclusion, child); e != nil {
		return false, e
	}
	d := m.declAt(a.decl.slot)
	if e = out.check(m); e != nil {
		return false, e
	}
	out.substitutions = append(out.substitutions, nil)
	if d.info.slot == 0 {
		return true, nil
	}
	info := m.infoAt(d.info.slot)
	for i := uint32(0); i < info.count; i++ {
		param, e := m.infoParameter(info, i)
		if e != nil {
			return false, e
		}
		value, e := m.parameter(param)
		if e != nil {
			return false, e
		}
		if value.Kind == ASTParameterInt {
			if i+1 >= info.count {
				if ASTSourceDebug {
					return false, ErrASTDomain
				}
				return false, arena.ErrBounds
			}
			next, e := m.infoParameter(info, i+1)
			if e != nil {
				return false, e
			}
			second, e := m.parameter(next)
			if e != nil {
				return false, e
			}
			if second.Kind != ASTParameterInt {
				return false, ErrASTDomain
			} // source inactive-union read is undefined
			out.positions = append(out.positions, ASTProofPosition{First: uint32(value.integer), Second: uint32(second.integer)})
			out.substitutions = append(out.substitutions, nil)
			i++
		} else {
			if value.Kind != ASTParameterAST {
				return false, ErrASTDomain
			}
			r := value.ast
			if _, e = m.requireExpr(m.external(r)); e != nil {
				return false, e
			}
			if e = m.incNode(r); e != nil {
				return false, e
			}
			last := len(out.substitutions) - 1
			out.substitutions[last] = append(out.substitutions[last], m.external(r))
		}
	}
	return true, nil
}
