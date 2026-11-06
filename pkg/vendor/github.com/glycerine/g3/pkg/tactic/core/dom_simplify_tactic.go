package core

// Source: src/tactic/core/dom_simplify_tactic.cpp/.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"reflect"
)

type DomSimplifyTactic struct {
	self       *DomSimplifyTactic
	manager    *ast.ASTManager
	dominators *expressionDominators
	maxDepth   uint32
	forward    bool
	traceTags  *diagnostic.TraceTags
	refs       uint32
	ownerBytes uint64
	closed     bool
	domSimp    DomSimplifier
	domCache   map[ast.ASTHandle]ast.ASTHandle
	domPins    []ast.ASTRef
}

// DomSimplifier is the callback contract used by the source's dominator
// simplification tactic. Assertion callbacks create one scope per assertion.
type DomSimplifier interface {
	AssertExpression(ast.ASTHandle, bool) (bool, error)
	Rewrite(ast.ASTRef) error
	Pop(uint32) error
	ScopeLevel() uint32
	TranslateDom(*ast.ASTManager) (DomSimplifier, error)
	UpdateParams(*uparams.ParamsRef) error
	Close() error
}

func (t *DomSimplifyTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if e := t.open(); e != nil {
		return e
	}
	t.traceTags = tags
	return nil
}

var _ tactic.Tactic = (*DomSimplifyTactic)(nil)

func NewDomSimplifyTactic(m *ast.ASTManager, _ *uparams.ParamsRef) (*DomSimplifyTactic, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(DomSimplifyTactic{}).Size())
	if e := m.Budget().Reserve(n); e != nil {
		return nil, e
	}
	t := &DomSimplifyTactic{manager: m, maxDepth: 1024, ownerBytes: n}
	t.self = t
	t.dominators = newExpressionDominators(m)
	return t, nil
}

func NewDomSimplifyTacticWithSimplifier(m *ast.ASTManager, s DomSimplifier, p *uparams.ParamsRef) (*DomSimplifyTactic, error) {
	t, err := NewDomSimplifyTactic(m, p)
	if err != nil {
		if s != nil {
			err = errors.Join(err, s.Close())
		}
		return nil, err
	}
	if s == nil {
		_ = t.Close()
		return nil, ast.ErrASTHandle
	}
	if p != nil {
		err = s.UpdateParams(p)
	}
	if err != nil {
		return nil, errors.Join(err, s.Close(), t.Close())
	}
	t.domSimp = s
	return t, nil
}
func (t *DomSimplifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *DomSimplifyTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *DomSimplifyTactic) DecRef() error {
	if e := t.open(); e != nil {
		return e
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *DomSimplifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.clearDomPins(); err != nil {
		return err
	}
	if t.domSimp != nil {
		if err := t.domSimp.Close(); err != nil {
			return err
		}
		t.domSimp = nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *DomSimplifyTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("dom-simplify", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	defer func() { err = errors.Join(err, t.clearDomPins()); t.domCache = nil }()
	if t.domSimp != nil {
		return t.applyDomSimplifier(g, out)
	}
	changed := true
	for changed {
		inconsistent, checkErr := g.Inconsistent()
		if checkErr != nil {
			return checkErr
		}
		if inconsistent {
			break
		}
		changed = false
		for _, forward := range []bool{true, false} {
			if e := t.clearDomPins(); e != nil {
				return e
			}
			t.domCache = nil
			t.forward = forward
			n, e := g.Size()
			if e != nil {
				return e
			}
			forms := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				forms[i], e = g.Form(i)
				if e != nil {
					return e
				}
			}
			root, e := domMkAndArgs(t.manager, forms)
			if e != nil {
				return e
			}
			if e = t.pinDom(root); e != nil {
				return e
			}
			compiled, e := t.dominators.compile(root)
			if e != nil {
				return e
			}
			if !compiled {
				if e = t.clearDomPins(); e != nil {
					return e
				}
				t.domCache = nil
				if e = g.IncDepth(); e != nil {
					return e
				}
				return out.AddGoal(g)
			}
			t.domCache = make(map[ast.ASTHandle]ast.ASTHandle)
			t.traceDominatorTree()
			ctx := &domContext{}
			order := make([]uint32, n)
			for i := range order {
				if forward {
					order[i] = uint32(i)
				} else {
					order[i] = n - 1 - uint32(i)
				}
			}
			for _, i := range order {
				inconsistent, e := g.Inconsistent()
				if e != nil {
					return e
				}
				if inconsistent {
					break
				}
				old, e := g.Form(i)
				if e != nil {
					return e
				}
				next, e := t.simplify(old, ctx, 0)
				if e != nil {
					return e
				}
				changed = changed || next != old
				if next != old {
					t.traceRewrite(old, next)
				}
				dep, e := g.Dependency(i)
				if e != nil {
					return e
				}
				proofs, e := g.ProofsEnabled()
				if e != nil {
					return e
				}
				if (forward && i+1 < n || !forward && i > 0) && !t.manager.IsTrue(next) && !t.manager.IsFalse(next) && dep.IsNull() && !proofs {
					if e = t.assertContext(ctx, next, false); e != nil {
						return e
					}
				}
				pr, e := g.Proof(i)
				if e != nil {
					return e
				}
				if proofs {
					rw, e := t.manager.MkProofRewrite(old, next)
					if e != nil {
						return e
					}
					pr, e = t.manager.MkProofModusPonens(pr, rw)
					if e != nil {
						return e
					}
				}
				if e = g.Update(i, next, pr, dep); e != nil {
					return e
				}
			}
			if e = t.clearDomPins(); e != nil {
				return e
			}
			t.domCache = nil
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}
func (t *DomSimplifyTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *DomSimplifyTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *DomSimplifyTactic) CollectStatistics() map[string]float64         { return nil }
func (t *DomSimplifyTactic) ResetStatistics() error                        { return t.open() }
func (t *DomSimplifyTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *DomSimplifyTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *DomSimplifyTactic) Cleanup() error {
	if e := t.open(); e != nil {
		return e
	}
	t.dominators.reset()
	t.domCache = nil
	return t.clearDomPins()
}
func (t *DomSimplifyTactic) Reset() error { return t.Cleanup() }
func (t *DomSimplifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	if t.domSimp != nil {
		s, err := t.domSimp.TranslateDom(m)
		if err != nil {
			return nil, err
		}
		return NewDomSimplifyTacticWithSimplifier(m, s, nil)
	}
	return NewDomSimplifyTactic(m, nil)
}
