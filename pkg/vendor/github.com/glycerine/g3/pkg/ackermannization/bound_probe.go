package ackermannization

// Source: src/ackermannization/ackr_bound_probe.cpp/.h.

import (
	"fmt"
	"io"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type BoundProbe struct {
	self   *BoundProbe
	budget *arena.Budget
	trace  *diagnostic.TraceTags
	header uint64
	refs   uint32
	closed bool
}

var _ tactic.ProbeEvaluator = (*BoundProbe)(nil)

func NewBoundProbe(b *arena.Budget) (*BoundProbe, error) {
	if b == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(BoundProbe{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	p := &BoundProbe{budget: b, header: n}
	p.self = p
	return p, nil
}
func (p *BoundProbe) Check() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return arena.ErrClosed
	}
	return nil
}
func (p *BoundProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}
func (p *BoundProbe) DecRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	if p.refs == 0 {
		return arena.ErrBounds
	}
	p.refs--
	if p.refs == 0 {
		return p.Close()
	}
	return nil
}
func (p *BoundProbe) Close() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return nil
	}
	p.budget.Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
func (p *BoundProbe) BindTrace(tags *diagnostic.TraceTags) error {
	if err := p.Check(); err != nil {
		return err
	}
	p.trace = tags
	return nil
}
func (p *BoundProbe) Eval(g *tactic.Goal) (float64, error) {
	if err := p.Check(); err != nil {
		return 0, err
	}
	if g == nil || g.Manager() == nil {
		return 0, ast.ErrASTHandle
	}
	m := g.Manager()
	if _, err := m.BitVectorUtilityFamilyID(); err != nil {
		return 0, err
	}
	groups := make(termGroups)
	seen := make(map[ast.ASTHandle]bool)
	n, err := g.Size()
	if err != nil {
		return 0, err
	}
	for i := uint32(0); i < n; i++ {
		h, err := g.Form(i)
		if err != nil {
			return 0, err
		}
		stack := []ast.ASTHandle{h}
		for len(stack) > 0 {
			h := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[h] {
				continue
			}
			seen[h] = true
			kind, err := m.GetKind(h)
			if err != nil {
				return 0, err
			}
			switch kind {
			case ast.ASTVar:
			case ast.ASTQuantifier:
				body, err := m.GetQuantifierBody(h)
				if err != nil {
					return 0, err
				}
				stack = append(stack, body) // Source ignores patterns.
			case ast.ASTApp:
				if err = groups.add(m, h); err != nil {
					return 0, err
				}
				n, err := m.GetNumArgs(h)
				if err != nil {
					return 0, err
				}
				for j := uint32(0); j < n; j++ {
					arg, err := m.GetArg(h, j)
					if err != nil {
						return 0, err
					}
					stack = append(stack, arg)
				}
			default:
				return 0, ast.ErrASTDomain
			}
		}
	}
	total := groups.bound()
	err = emitAck(p.trace, "ackr_bound_probe", "operator()", "src/ackermannization/ackr_bound_probe.cpp", 67, func(w io.Writer) error {
		if math.IsInf(total, 1) {
			_, e := io.WriteString(w, "total=inf\n")
			return e
		}
		_, e := fmt.Fprintf(w, "total=%.6g\n", total)
		return e
	})
	return total, err
}
