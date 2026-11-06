package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp assert_expr_core/pop_core.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type bvBoundsUndo struct {
	key   ast.ASTHandle
	ref   ast.ASTRef
	old   bvBoundsSet
	fresh bool
}
type BVBoundsSimplifier struct {
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	occurrences *tactic.GoalNumOccurs
	bounds      map[ast.ASTHandle]bvBoundsSet
	trail       []bvBoundsUndo
	propagateEq bool
	closed      bool
}

func NewBVBoundsSimplifier(m *ast.ASTManager, c *uparams.ParamsContext) *BVBoundsSimplifier {
	return &BVBoundsSimplifier{manager: m, context: c, bounds: make(map[ast.ASTHandle]bvBoundsSet)}
}
func (s *BVBoundsSimplifier) SetOccurrences(n *tactic.GoalNumOccurs) { s.occurrences = n }
func (s *BVBoundsSimplifier) ScopeLevel() uint32                     { return uint32(len(s.trail)) }
func (s *BVBoundsSimplifier) AssertExpression(h ast.ASTHandle, sign bool) (bool, error) {
	for {
		not, arg, e := s.manager.IsNot(h)
		if e != nil {
			return false, e
		}
		if !not {
			break
		}
		h = arg
		sign = !sign
	}
	key, b, ok, e := s.recognize(h)
	if e != nil || !ok {
		return e == nil, e
	}
	if sign {
		b = b.sourceComplement()
	}
	if old, exists := s.bounds[key]; exists {
		next := old.intersect(b)
		if next.empty() {
			return false, nil
		}
		if next.equal(old) {
			return true, nil
		}
		ref, e := s.manager.NewRef(key)
		if e != nil {
			return false, e
		}
		s.trail = append(s.trail, bvBoundsUndo{key: key, ref: ref, old: old})
		s.bounds[key] = next
	} else {
		ref, e := s.manager.NewRef(key)
		if e != nil {
			return false, e
		}
		s.trail = append(s.trail, bvBoundsUndo{key: key, ref: ref, fresh: true})
		s.bounds[key] = b
	}
	return true, nil
}
func (s *BVBoundsSimplifier) Pop(n uint32) (err error) {
	if n > uint32(len(s.trail)) {
		return ast.ErrASTDomain
	}
	for ; n > 0; n-- {
		i := len(s.trail) - 1
		u := s.trail[i]
		if u.fresh {
			delete(s.bounds, u.key)
		} else {
			s.bounds[u.key] = u.old
		}
		err = errors.Join(err, s.manager.DelRef(&u.ref))
		s.trail = s.trail[:i]
	}
	return err
}
func (s *BVBoundsSimplifier) Close() error {
	if s.closed {
		return nil
	}
	if e := s.Pop(s.ScopeLevel()); e != nil {
		return e
	}
	s.closed = true
	return nil
}
