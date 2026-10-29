package rewriter

// Source: src/ast/rewriter/rewriter_def.h:221-552, 644-750.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *rewriterEngine) transitivity(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return r.core.manager.MkProofTransitivity(a, b)
}

func (r *rewriterEngine) replaceProofSuffix(spos uint32, proof ast.ASTHandle) error {
	if e := shrinkRewriterRoots(r.core.manager, &r.core.proofResults, spos); e != nil {
		return e
	}
	return r.appendProof(proof)
}

func (r *rewriterEngine) finishProof(fr rewriterFrame, value, proof ast.ASTHandle, unconditional bool) (err error) {
	c, m := &r.core, r.core.manager
	if e := m.SetRef(r.r, value); e != nil {
		return e
	}
	if e := m.SetRef(r.pr, proof); e != nil {
		return e
	}
	defer func() { err = errors.Join(err, m.ResetRef(r.pr), m.ResetRef(r.r)) }()
	if e := shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
		return e
	}
	if e := appendRewriterRoot(m, &c.results, value); e != nil {
		return e
	}
	if e := r.replaceProofSuffix(fr.spos, proof); e != nil {
		return e
	}
	if e := r.cache(fr, value); e != nil {
		return e
	}
	c.frames = c.frames[:len(c.frames)-1]
	if unconditional {
		return r.setNewChild(fr.curr)
	}
	return r.setNewChildIfDifferent(fr.curr, value)
}

func (r *rewriterEngine) rewrittenProof(fr rewriterFrame) (err error) {
	c, m := &r.core, r.core.manager
	if len(c.results) < 2 || len(c.proofResults) < 2 {
		return ast.ErrASTDomain
	}
	value := c.results[len(c.results)-1]
	p1 := c.proofResults[len(c.proofResults)-2]
	p2 := c.proofResults[len(c.proofResults)-1]
	proof, e := r.transitivity(p1, p2)
	if e != nil {
		return e
	}
	if e = m.SetRef(r.r, value); e != nil {
		return e
	}
	if e = m.SetRef(r.pr, proof); e != nil {
		return e
	}
	defer func() { err = errors.Join(err, m.ResetRef(r.pr), m.ResetRef(r.r)) }()
	if e = shrinkRewriterRoots(m, &c.results, uint32(len(c.results)-2)); e != nil {
		return e
	}
	if e = appendRewriterRoot(m, &c.results, value); e != nil {
		return e
	}
	if e = shrinkRewriterRoots(m, &c.proofResults, uint32(len(c.proofResults)-2)); e != nil {
		return e
	}
	if e = r.appendProof(proof); e != nil {
		return e
	}
	if e = r.cache(fr, value); e != nil {
		return e
	}
	c.frames = c.frames[:len(c.frames)-1]
	return r.setNewChild(fr.curr)
}

func (r *rewriterEngine) processAppProof() error {
	c, m := &r.core, r.core.manager
	fi := len(c.frames) - 1
	fr := c.frames[fi]
	t := fr.curr
	n, e := m.GetNumArgs(t)
	if e != nil {
		return e
	}
	switch fr.state() {
	case 0:
		for c.frames[fi].index() < n {
			i := c.frames[fi].index()
			arg, e := m.GetArg(t, i)
			if e != nil {
				return e
			}
			c.frames[fi].setIndex(i + 1)
			done, e := r.visit(arg, fr.maxDepth())
			if e != nil || !done {
				return e
			}
		}
		fr = c.frames[fi]
		d, e := m.GetDecl(t)
		if e != nil {
			return e
		}
		size := uint32(len(c.results))
		args, e := c.resultArguments(fr.spos, size-fr.spos)
		if e != nil {
			return e
		}
		if e = c.elimReflexProofs(fr.spos); e != nil {
			return e
		}
		proof := ast.ASTHandle{}
		newTerm := t
		if uint32(len(c.proofResults)) > fr.spos {
			newTerm, e = m.RewriterMakeApp(d, args)
			if e != nil {
				return e
			}
			ps := append([]ast.ASTHandle(nil), c.proofResults[fr.spos:]...)
			proof, e = m.MkProofCongruence(t, newTerm, ps)
			if e != nil {
				return e
			}
		}
		st, e := r.cfg.reduceApp(d, args, r.r, r.pr2)
		if e != nil {
			return e
		}
		if st != brFailed {
			value, e := m.RefGet(r.r)
			if e != nil {
				return e
			}
			if e = r.sameSort(t, value); e != nil {
				return e
			}
			step, e := m.RefGet(r.pr2)
			if e != nil {
				return e
			}
			if step.IsNull() {
				step, e = m.MkProofRewrite(newTerm, value)
				if e != nil {
					return e
				}
			}
			proof, e = r.transitivity(proof, step)
			if e != nil {
				return e
			}
			// Transitivity may return the step itself when the first proof is
			// null. Pin it before releasing the callback's proof reference.
			if e = m.SetRef(r.pr, proof); e != nil {
				return e
			}
			if e = m.ResetRef(r.pr2); e != nil {
				return e
			}
			if e = shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
				return e
			}
			if e = appendRewriterRoot(m, &c.results, value); e != nil {
				return e
			}
			if e = r.replaceProofSuffix(fr.spos, proof); e != nil {
				return e
			}
			if e = m.ResetRef(r.pr); e != nil {
				return e
			}
			if st == brDone {
				if e = r.cache(fr, value); e != nil {
					return e
				}
				c.frames = c.frames[:fi]
				if e = r.setNewChild(t); e != nil {
					return e
				}
				return m.ResetRef(r.r)
			}
			if st > brRewriteFull {
				return ast.ErrASTDomain
			}
			c.frames[fi].setState(1)
			depth := uint32(st)
			if depth != 3 {
				depth++
			}
			done, e := r.visit(value, depth)
			if e != nil {
				return e
			}
			if done {
				e = r.rewrittenProof(fr)
			}
			if reset := m.ResetRef(r.r); e == nil {
				e = reset
			}
			return e
		}
		def, _, defProof, macro, e := r.cfg.getMacro(d)
		if e != nil {
			return e
		}
		value := newTerm
		if macro {
			if e = r.sameSort(t, def); e != nil {
				return e
			}
			ground, e := m.IsGround(def)
			if e != nil {
				return e
			}
			if !ground {
				// The source proof specialization deliberately leaves
				// non-ground macro expansion unimplemented.
				return ast.ErrASTDomain
			}
			value = def
			proof, e = r.transitivity(proof, defProof)
			if e != nil {
				return e
			}
		}
		return r.finishProof(fr, value, proof, false)
	case 1:
		return r.rewrittenProof(fr)
	default:
		return ast.ErrASTDomain
	}
}

func (r *rewriterEngine) processQuantifierProof() (err error) {
	c, m := &r.core, r.core.manager
	fi := len(c.frames) - 1
	fr := c.frames[fi]
	q := fr.curr
	nd, e := m.GetNumDeclarations(q)
	if e != nil {
		return e
	}
	np, e := m.GetNumPatterns(q)
	if e != nil {
		return e
	}
	nn, e := m.GetNumNoPatterns(q)
	if e != nil {
		return e
	}
	body, e := m.GetQuantifierBody(q)
	if e != nil {
		return e
	}
	if fr.index() == 0 {
		c.numQVars += nd
	}
	children := uint32(1)
	if r.cfg.rewritePatterns() {
		children += np + nn
	}
	for c.frames[fi].index() < children {
		i := c.frames[fi].index()
		h := body
		if i > 0 {
			if i <= np {
				h, e = m.GetPattern(q, i-1)
			} else {
				h, e = m.GetNoPattern(q, i-1-np)
			}
			if e != nil {
				return e
			}
		}
		c.frames[fi].setIndex(i + 1)
		done, e := r.visit(h, fr.maxDepth())
		if e != nil || !done {
			return e
		}
	}
	fr = c.frames[fi]
	if len(r.patterns) != 0 || len(r.noPatterns) != 0 {
		return ast.ErrASTPending
	}
	defer func() { err = errors.Join(err, r.releasePatterns()) }()
	for i := uint32(0); i < np; i++ {
		h, e := m.GetPattern(q, i)
		if e != nil {
			return e
		}
		if e = appendRewriterRoot(m, &r.patterns, h); e != nil {
			return e
		}
	}
	for i := uint32(0); i < nn; i++ {
		h, e := m.GetNoPattern(q, i)
		if e != nil {
			return e
		}
		if e = appendRewriterRoot(m, &r.noPatterns, h); e != nil {
			return e
		}
	}
	if r.cfg.rewritePatterns() {
		for i := uint32(0); i < np+nn; i++ {
			h, e := c.resultAt(fr.spos + 1 + i)
			if e != nil {
				return e
			}
			pattern, e := m.RewriterPattern(h)
			if e != nil {
				return e
			}
			if pattern {
				v, j := &r.patterns, i
				if i >= np {
					v, j = &r.noPatterns, i-np
				}
				if e = replaceRewriterRoot(m, v, j, h); e != nil {
					return e
				}
			}
		}
	}
	newBody, e := c.resultAt(fr.spos)
	if e != nil {
		return e
	}
	bodyProof := c.proofResults[fr.spos]
	pa, na := ast.SliceArguments(r.patterns), ast.SliceArguments(r.noPatterns)
	newQ, e := m.UpdateQuantifierArguments(q, newBody, pa, na)
	if e != nil {
		return e
	}
	proof := ast.ASTHandle{}
	if newQ != q {
		proof, e = m.MkProofQuantIntro(q, newQ, bodyProof)
		if e != nil {
			return e
		}
	}
	if e = m.SetRef(r.r, newQ); e != nil {
		return e
	}
	done, e := r.cfg.reduceQuantifier(newQ, newBody, pa, na, r.r, r.pr2)
	if e != nil {
		return e
	}
	if done {
		step, e := m.RefGet(r.pr2)
		if e != nil {
			return e
		}
		proof, e = r.transitivity(proof, step)
		if e != nil {
			return e
		}
	}
	value, e := m.RefGet(r.r)
	if e != nil {
		return e
	}
	if e = r.finishProof(fr, value, proof, true); e != nil {
		return e
	}
	if e = m.ResetRef(r.pr2); e != nil {
		return e
	}
	return m.ResetRef(r.r)
}
