package ackermannization

// Source: src/ackermannization/lackr.cpp eager encoding path.

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

type Stats struct{ Iterations, Constraints uint32 }

func (s *Stats) Reset() { s.Iterations = 0; s.Constraints = 0 }

type eagerEncoder struct {
	owner                     *encodingState
	forms, abstracted, lemmas []ast.ASTHandle
	held, scratch             []ast.ASTHandle
	groups                    termGroups
	info                      *Info
	infoCleanup               bool
	simplifier                *rewriter.TheoryRewriter
	out, simplified           ast.ASTRef
	eager                     bool
	initialized               bool
	failures                  []memory.ConstructionFailure
}

func (r *eagerEncoder) hold(h ast.ASTHandle, scratch bool) error {
	if err := r.owner.manager.IncRef(h); err != nil {
		return err
	}
	if scratch {
		r.scratch = append(r.scratch, h)
	} else {
		r.held = append(r.held, h)
	}
	return nil
}
func (r *eagerEncoder) releaseScratch(limit int) error {
	for len(r.scratch) > limit {
		i := len(r.scratch) - 1
		if err := r.owner.manager.DecRef(r.scratch[i]); err != nil {
			return err
		}
		r.scratch = r.scratch[:i]
	}
	return nil
}
func (r *eagerEncoder) emit(function string, line int, body func(io.Writer) error) error {
	return emitAck(r.owner.trace, "lackr", function, "src/ackermannization/lackr.cpp", line, body)
}
func (r *eagerEncoder) write(w io.Writer, h ast.ASTHandle) error {
	return writeAckExpr(r.owner.manager, r.owner.context, r.owner.globals, w, h)
}
func (r *eagerEncoder) label(function string, line int, label string, h ast.ASTHandle) error {
	return r.emit(function, line, func(w io.Writer) error {
		if _, err := io.WriteString(w, label); err != nil {
			return err
		}
		if err := r.write(w, h); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (r *eagerEncoder) construct(g *tactic.Goal) error {
	n, err := g.Size()
	if err != nil {
		return err
	}
	var forms []ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		f, err := g.Form(i)
		if err != nil {
			return err
		}
		forms = append(forms, f)
	}
	return r.constructForms(forms)
}
func (r *eagerEncoder) constructForms(forms []ast.ASTHandle) error {
	m := r.owner.manager
	for _, f := range forms {
		if err := r.hold(f, false); err != nil {
			return err
		}
		r.forms = append(r.forms, f)
	}
	var err error
	r.simplifier, err = rewriter.NewTheoryRewriter(m, r.owner.context, r.owner.globals, nil)
	if err != nil {
		return err
	}
	r.eager, err = r.owner.readEager(&r.owner.params)
	if err != nil {
		return err
	}
	r.out, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	r.simplified, err = m.NewRef(ast.ASTHandle{})
	return err
}
func (r *eagerEncoder) collect() (bool, error) {
	m := r.owner.manager
	seen := make(map[ast.ASTHandle]bool)
	stack := append([]ast.ASTHandle(nil), r.forms...)
	for _, f := range r.forms {
		if err := r.label("collect_terms", 257, "infla: ", f); err != nil {
			return false, err
		}
	}
	for len(stack) > 0 {
		term := stack[len(stack)-1]
		if seen[term] {
			stack = stack[:len(stack)-1]
			continue
		}
		kind, err := m.GetKind(term)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTVar:
			seen[term] = true
			stack = stack[:len(stack)-1]
		case ast.ASTQuantifier:
			return false, nil
		case ast.ASTApp:
			n, err := m.GetNumArgs(term)
			if err != nil {
				return false, err
			}
			ready := true
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(term, i)
				if err != nil {
					return false, err
				}
				if !seen[arg] {
					stack = append(stack, arg)
					ready = false
				}
			}
			if !ready {
				continue
			}
			seen[term] = true
			stack = stack[:len(stack)-1]
			if n != 0 {
				yes, err := ShouldAckermannize(m, term)
				if err != nil {
					return false, err
				}
				if yes {
					if err = r.emit("add_term", 193, func(w io.Writer) error {
						if _, e := io.WriteString(w, "term("); e != nil {
							return e
						}
						if e := r.write(w, term); e != nil {
							return e
						}
						_, e := io.WriteString(w, ")\n")
						return e
					}); err != nil {
						return false, err
					}
					if err = r.groups.add(m, term); err != nil {
						return false, err
					}
				}
			}
		default:
			return false, ast.ErrASTDomain
		}
	}
	return true, nil
}
func (r *eagerEncoder) abstract() error {
	m := r.owner.manager
	for d, terms := range r.groups {
		sort, err := m.GetRange(d)
		if err != nil {
			return err
		}
		name, err := m.GetASTName(d)
		if err != nil {
			return err
		}
		prefix, err := m.Symbols.String(name)
		if err != nil {
			return err
		}
		for term := range terms {
			fresh, err := m.MkFreshConstNullable(&prefix, sort)
			if err != nil {
				return err
			}
			if err = r.info.SetAbstraction(term, fresh); err != nil {
				return err
			}
			if err = r.emit("abstract", 167, func(w io.Writer) error {
				if _, e := io.WriteString(w, "abstr term "); e != nil {
					return e
				}
				if e := r.write(w, term); e != nil {
					return e
				}
				if _, e := io.WriteString(w, " -> "); e != nil {
					return e
				}
				if e := r.write(w, fresh); e != nil {
					return e
				}
				_, e := io.WriteString(w, "\n")
				return e
			}); err != nil {
				return err
			}
		}
	}
	if err := r.info.Seal(); err != nil {
		return err
	}
	for _, f := range r.forms {
		if err := r.info.Abstract(f, r.out); err != nil {
			return err
		}
		h, err := m.RefGet(r.out)
		if err != nil {
			return err
		}
		if err = r.hold(h, false); err != nil {
			return err
		}
		r.abstracted = append(r.abstracted, h)
	}
	return nil
}
func (r *eagerEncoder) ackermann(first, second ast.ASTHandle) (err error) {
	m := r.owner.manager
	start := len(r.scratch)
	defer func() { err = errors.Join(err, r.releaseScratch(start)) }()
	if err = r.emit("ackr", 92, func(w io.Writer) error {
		if _, e := io.WriteString(w, "ackr "); e != nil {
			return e
		}
		if e := r.write(w, first); e != nil {
			return e
		}
		if _, e := io.WriteString(w, " , "); e != nil {
			return e
		}
		if e := r.write(w, second); e != nil {
			return e
		}
		_, e := io.WriteString(w, "\n")
		return e
	}); err != nil {
		return err
	}
	n, err := m.GetNumArgs(first)
	if err != nil {
		return err
	}
	var equalities []ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		a, e := m.GetArg(first, i)
		if e != nil {
			return e
		}
		b, e := m.GetArg(second, i)
		if e != nil {
			return e
		}
		same, e := m.AreEqual(a, b)
		if e != nil {
			return e
		}
		if same {
			continue
		}
		different, e := m.AreDistinct(a, b)
		if e != nil {
			return e
		}
		if different {
			return r.emit("ackr", 102, func(w io.Writer) error { _, e := io.WriteString(w, "never eq\n"); return e })
		}
		eq, e := m.MkEq(a, b)
		if e != nil {
			return e
		}
		if e = r.hold(eq, true); e != nil {
			return e
		}
		equalities = append(equalities, eq)
	}
	a, err := r.info.Abstraction(first)
	if err != nil {
		return err
	}
	b, err := r.info.Abstraction(second)
	if err != nil {
		return err
	}
	if err = r.label("ackr", 110, "abstr1 ", a); err != nil {
		return err
	}
	if err = r.label("ackr", 111, "abstr2 ", b); err != nil {
		return err
	}
	lhs, err := m.MkAndArgs(equalities)
	if err != nil {
		return err
	}
	if err = r.hold(lhs, true); err != nil {
		return err
	}
	if err = r.label("ackr", 114, "ackr constr lhs", lhs); err != nil {
		return err
	}
	rhs, err := m.MkEq(a, b)
	if err != nil {
		return err
	}
	if err = r.hold(rhs, true); err != nil {
		return err
	}
	if err = r.label("ackr", 116, "ackr constr rhs", rhs); err != nil {
		return err
	}
	formula, err := m.MkImplies(lhs, rhs)
	if err != nil {
		return err
	}
	if err = r.hold(formula, true); err != nil {
		return err
	}
	if err = r.label("ackr", 118, "ackr constr", formula); err != nil {
		return err
	}
	if err = r.info.Abstract(formula, r.out); err != nil {
		return err
	}
	abstract, err := m.RefGet(r.out)
	if err != nil {
		return err
	}
	if err = r.simplifier.Rewrite(abstract, r.simplified); err != nil {
		return err
	}
	lemma, err := m.RefGet(r.simplified)
	if err != nil {
		return err
	}
	if err = r.label("ackr", 122, "ackr constr abs:", lemma); err != nil {
		return err
	}
	if m.IsTrue(lemma) {
		return nil
	}
	r.owner.stats.Constraints++
	if err = r.hold(lemma, false); err != nil {
		return err
	}
	r.lemmas = append(r.lemmas, lemma)
	return nil
}
func (r *eagerEncoder) encode() error {
	if err := r.emit("eager_enc", 133, func(w io.Writer) error { _, e := fmt.Fprintf(w, "#funs: %d\n", len(r.groups)); return e }); err != nil {
		return err
	}
	for _, terms := range r.groups {
		canceled, err := r.owner.manager.Canceled()
		if err != nil {
			return err
		}
		if canceled {
			return tactic.NewTacticException(tactic.TacticCanceledMessage)
		}
		if r.owner.services.Cooperation != nil {
			if err = r.owner.services.Cooperation.Cooperate("lackr"); err != nil {
				return err
			}
		}
		var list []ast.ASTHandle
		for t := range terms {
			list = append(list, t)
		}
		for i, first := range list {
			for _, second := range list[i+1:] {
				if first != second {
					if err = r.ackermann(first, second); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
func (r *eagerEncoder) mkAckermann(g *tactic.Goal, limit float64) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	initialized, err := r.initialize()
	if err != nil || !initialized {
		return false, err
	}
	if !math.IsInf(limit, 1) && r.groups.bound() > limit {
		return false, nil
	}
	if err = r.encode(); err != nil {
		return false, err
	}
	for _, h := range r.abstracted {
		if err = g.Assert(h); err != nil {
			return false, err
		}
	}
	for _, h := range r.lemmas {
		if err = g.Assert(h); err != nil {
			return false, err
		}
	}
	return true, nil
}
func (r *eagerEncoder) initialize() (bool, error) {
	if ast.ASTSourceDebug && r.initialized {
		return false, ast.ErrASTDomain
	}
	if err := r.simplifier.UpdateParams(&r.owner.params); err != nil {
		return false, err
	}
	info, err := NewInfo(r.owner.manager)
	if err != nil {
		return false, err
	}
	r.info = info
	if err = info.IncRef(); err != nil {
		return false, err
	}
	collected, err := r.collect()
	if err != nil || !collected {
		return false, err
	}
	if err = r.abstract(); err != nil {
		return false, err
	}
	r.initialized = true
	return true, nil
}
func (r *eagerEncoder) Close() error {
	for len(r.failures) > 0 {
		i := len(r.failures) - 1
		if err := r.failures[i].Cleanup(); err != nil {
			return err
		}
		r.failures = r.failures[:i]
	}
	if r.simplifier != nil {
		if err := r.simplifier.Close(); err != nil {
			return err
		}
		r.simplifier = nil
	}
	for _, ref := range []*ast.ASTRef{&r.simplified, &r.out} {
		if *ref != (ast.ASTRef{}) {
			if err := r.owner.manager.DelRef(ref); err != nil {
				return err
			}
			*ref = ast.ASTRef{}
		}
	}
	if r.info != nil {
		if r.infoCleanup {
			if err := r.info.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := r.info.DecRefConsumed()
			if consumed && err != nil {
				r.infoCleanup = true
			}
			if err != nil {
				return err
			}
		}
		r.info = nil
	}
	if err := r.releaseScratch(0); err != nil {
		return err
	}
	for len(r.held) > 0 {
		i := len(r.held) - 1
		if err := r.owner.manager.DecRef(r.held[i]); err != nil {
			return err
		}
		r.held = r.held[:i]
	}
	return nil
}
