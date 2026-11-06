package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrUnresolvedITE = errors.New("unresolved ite in model")
var ErrUnresolvedIff = errors.New("disaster in SubtermTruth processing equivalence")

// implicantState owns generated expressions until its returned conjunction has
// acquired an independent reference. Input handles borrow the live root.
type implicantState struct {
	graph    *RPFP
	truth    map[ast.ASTHandle]int
	done     [2]map[ast.ASTHandle]bool
	resolved map[ast.ASTHandle]ast.ASTHandle
	ignore   map[ast.ASTHandle]bool
	pins     []*Expr
	literals []ast.ASTHandle
}

func (s *implicantState) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pin, err := NewExpr(s.graph.context, h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.pins = append(s.pins, pin)
	return h, nil
}
func (s *implicantState) close() (result error) {
	for _, pin := range s.pins {
		result = errors.Join(result, pin.Close())
	}
	return
}

// UnderapproxFormula follows ImplicantRed, including the source equivalence
// rule that records only the right operand's selected polarity.
// Source: duality_rpfp.cpp:1746-1820,1853-1889,1940-1963.
func (g *RPFP) UnderapproxFormula(formula *Expr, dontCares []*Expr) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	state := &implicantState{graph: g, truth: make(map[ast.ASTHandle]int), resolved: make(map[ast.ASTHandle]ast.ASTHandle), ignore: make(map[ast.ASTHandle]bool)}
	state.done[0] = make(map[ast.ASTHandle]bool)
	state.done[1] = make(map[ast.ASTHandle]bool)
	defer func() {
		result = errors.Join(result, state.close())
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	for _, term := range dontCares {
		if err := term.open(); err != nil {
			return nil, err
		}
		if term.value.object.context != g.context {
			return nil, ErrContextMismatch
		}
		h, err := term.Raw()
		if err != nil {
			return nil, err
		}
		state.ignore[h] = true
	}
	h, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	if err = state.reduce(h, true); err != nil {
		return nil, err
	}
	m := g.context.manager
	var combined ast.ASTHandle
	switch len(state.literals) {
	case 0:
		combined, err = m.MkTrue()
	case 1:
		combined = state.literals[0]
	default:
		combined, err = m.MkAndArgs(state.literals)
	}
	if err != nil {
		return nil, err
	}
	return NewExpr(g.context, combined)
}
func (s *implicantState) reduce(h ast.ASTHandle, positive bool) (result error) {
	polarity := 0
	if positive {
		polarity = 1
	}
	if s.done[polarity][h] {
		return nil
	}
	defer func() {
		if result == nil {
			s.done[polarity][h] = true
		}
	}()
	m := s.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(h)
		if err != nil {
			return err
		}
		if family == m.BasicFamilyID() {
			op, err := m.GetDeclKind(h)
			if err != nil {
				return err
			}
			switch op {
			case ast.ASTOpImplies:
				a, err := m.GetArg(h, 0)
				if err != nil {
					return err
				}
				b, err := m.GetArg(h, 1)
				if err != nil {
					return err
				}
				neg, err := s.keep(m.MkNot(a))
				if err != nil {
					return err
				}
				rewritten, err := s.keep(m.MkOr(b, neg))
				if err != nil {
					return err
				}
				return s.reduce(rewritten, positive)
			case ast.ASTOpIff:
				a, err := m.GetArg(h, 0)
				if err != nil {
					return err
				}
				value, err := s.graph.subtermTruth(s.truth, a)
				if err != nil {
					return err
				}
				if value == 2 {
					return ErrUnresolvedIff
				}
				b, err := m.GetArg(h, 1)
				if err != nil {
					return err
				}
				selected := value == 1
				if !positive {
					selected = !selected
				}
				return s.reduce(b, selected)
			case ast.ASTOpAnd, ast.ASTOpOr:
				all := (positive && op == ast.ASTOpAnd) || (!positive && op == ast.ASTOpOr)
				n, err := m.GetNumArgs(h)
				if err != nil {
					return err
				}
				for i := uint32(0); i < n; i++ {
					child, err := m.GetArg(h, i)
					if err != nil {
						return err
					}
					if !all {
						value, err := s.graph.timedSubtermTruth(s.truth, child)
						if err != nil {
							return err
						}
						if (positive && value != 1) || (!positive && value != 0) {
							continue
						}
					}
					if err = s.reduce(child, positive); err != nil {
						return err
					}
					if !all {
						return nil
					}
				}
				// Source silently emits no literal if no branch matches the model.
				return nil
			case ast.ASTOpNot:
				child, err := m.GetArg(h, 0)
				if err != nil {
					return err
				}
				return s.reduce(child, !positive)
			}
		}
	}
	if s.ignore[h] {
		return nil
	}
	resolved, err := s.resolve(h)
	if err != nil {
		return err
	}
	if !positive {
		resolved, err = s.keep(m.MkNot(resolved))
		if err != nil {
			return err
		}
	}
	s.literals = append(s.literals, resolved)
	return nil
}
func (s *implicantState) resolve(h ast.ASTHandle) (ast.ASTHandle, error) {
	if found, ok := s.resolved[h]; ok {
		return found, nil
	}
	m := s.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := h
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if family == m.BasicFamilyID() && op == ast.ASTOpIte {
			cond, err := m.GetArg(h, 0)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			value, err := s.graph.timedSubtermTruth(s.truth, cond)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if value == 2 {
				return ast.ASTHandle{}, ErrUnresolvedITE
			}
			if err = s.reduce(cond, value == 1); err != nil {
				return ast.ASTHandle{}, err
			}
			index := uint32(2)
			if value == 1 {
				index = 1
			}
			branch, err := m.GetArg(h, index)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = s.resolve(branch)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		} else {
			n, err := m.GetNumArgs(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(h, i)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				args[i], err = s.resolve(arg)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			decl, err := m.GetDecl(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = s.keep(m.MkApp(decl, args))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	s.resolved[h] = result
	return result, nil
}

// FixCurrentState records model-selected edge decisions in the current graph
// scope; the caller pushes that scope before expanding the next search level.
// Source: duality_rpfp.cpp:2519-2527.
func (g *RPFP) FixCurrentState(edge *Edge) (result error) {
	if err := g.solverReady(); err != nil {
		return err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return ErrWrapperOwner
	}
	m := g.context.manager
	var h ast.ASTHandle
	var err error
	if edge.dual == (ast.ASTRef{}) {
		h, err = m.MkTrue()
	} else {
		h, err = m.RefGet(edge.dual)
	}
	if err != nil {
		return err
	}
	formula, err := NewExpr(g.context, h)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	// Underapproximation flags are temporary query controls. Fixing their
	// model polarity can contradict the next expansion-selection assumptions.
	var controls []*Expr
	defer func() {
		for _, control := range controls {
			result = errors.Join(result, control.Close())
		}
	}()
	for _, ref := range g.underapproxFlagRefs {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		control, err := NewExpr(g.context, h)
		if err != nil {
			return err
		}
		controls = append(controls, control)
	}
	timer, err := startProfile(g.context, "UnderapproxFormula")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	decision, err := g.UnderapproxFormula(formula, controls)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, decision.Close()) }()
	if err = timer.Close(); err != nil {
		return err
	}
	return g.ConstrainEdgeLocalized(edge, decision)
}
