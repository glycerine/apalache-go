package interp

// Source: src/interp/iz3scopes.cpp; src/interp/iz3scopes.h.

import "fmt"

const (
	ScopeBottom = -1 << 15
	ScopeTop    = 1<<15 - 1
)

// ScopeRange is the compact interval representation selected by the source
// build. Lo and Hi are tree nodes, with ScopeBottom and ScopeTop as sentinels.
type ScopeRange struct {
	Lo int
	Hi int
}

// Scopes owns a copy of the parent relation. An empty relation selects the
// source's linear sequence mode.
type Scopes struct {
	parents []int
}

func NewScopes(parents []int) (*Scopes, error) {
	s := &Scopes{}
	if err := s.Initialize(parents); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Scopes) Initialize(parents []int) error {
	for node, parent := range parents {
		if parent != ScopeTop && (parent <= node || parent >= len(parents)) {
			return fmt.Errorf("interp: invalid parent %d for scope node %d", parent, node)
		}
	}
	s.parents = append(s.parents[:0], parents...)
	return nil
}

func (s *Scopes) TreeMode() bool { return s != nil && len(s.parents) != 0 }

func (s *Scopes) treeLCA(n1, n2 int) (int, error) {
	if s == nil {
		return 0, NewError("invalid interpolation scopes")
	}
	if !s.TreeMode() {
		return max(n1, n2), nil
	}
	if n1 == ScopeBottom {
		return n2, nil
	}
	if n2 == ScopeBottom {
		return n1, nil
	}
	if n1 == ScopeTop || n2 == ScopeTop {
		return ScopeTop, nil
	}
	for n1 != n2 {
		if n1 == ScopeTop || n2 == ScopeTop {
			return ScopeTop, nil
		}
		if n1 < 0 || n2 < 0 || n1 >= len(s.parents) || n2 >= len(s.parents) {
			return 0, fmt.Errorf("interp: scope node outside parent relation: %d, %d", n1, n2)
		}
		if n1 < n2 {
			n1 = s.parents[n1]
		} else {
			n2 = s.parents[n2]
		}
	}
	return n1, nil
}

func (s *Scopes) treeGCD(n1, n2 int) (int, error) {
	if s == nil {
		return 0, NewError("invalid interpolation scopes")
	}
	if !s.TreeMode() {
		return min(n1, n2), nil
	}
	lca, err := s.treeLCA(n1, n2)
	if err != nil {
		return 0, err
	}
	if lca == n1 {
		return n2, nil
	}
	if lca == n2 {
		return n1, nil
	}
	return ScopeBottom, nil
}

func EmptyScopeRange() ScopeRange { return ScopeRange{Lo: ScopeTop, Hi: ScopeBottom} }
func FullScopeRange() ScopeRange  { return ScopeRange{Lo: ScopeBottom, Hi: ScopeTop} }
func DownwardScopeRange(hi int) ScopeRange {
	return ScopeRange{Lo: ScopeBottom, Hi: hi}
}
func (r ScopeRange) IsEmpty() bool { return r.Hi < r.Lo }
func (r ScopeRange) IsFull() bool  { return r.Lo == ScopeBottom && r.Hi == ScopeTop }
func (r ScopeRange) Max() int      { return r.Hi }
func (r ScopeRange) Min() int      { return r.Lo }

func (s *Scopes) RangeLUB(a, b ScopeRange) (ScopeRange, error) {
	lo, err := s.treeGCD(a.Lo, b.Lo)
	if err != nil {
		return ScopeRange{}, err
	}
	hi, err := s.treeLCA(a.Hi, b.Hi)
	if err != nil {
		return ScopeRange{}, err
	}
	return ScopeRange{Lo: lo, Hi: hi}, nil
}

func (s *Scopes) RangeGLB(a, b ScopeRange) (ScopeRange, error) {
	lo, err := s.treeLCA(a.Lo, b.Lo)
	if err != nil {
		return ScopeRange{}, err
	}
	hi, err := s.treeGCD(a.Hi, b.Hi)
	if err != nil {
		return ScopeRange{}, err
	}
	return ScopeRange{Lo: lo, Hi: hi}, nil
}

func (s *Scopes) RangeAdd(node int, r *ScopeRange) error {
	if r == nil {
		return NewError("nil interpolation scope range")
	}
	added, err := s.RangeLUB(ScopeRange{Lo: node, Hi: node}, *r)
	if err != nil {
		return err
	}
	*r = added
	return nil
}

func (s *Scopes) InRange(node int, r ScopeRange) (bool, error) {
	lo, err := s.treeLCA(r.Lo, node)
	if err != nil {
		return false, err
	}
	hi, err := s.treeGCD(r.Hi, node)
	return err == nil && lo == node && hi == node, err
}

func (s *Scopes) RangesIntersect(a, b ScopeRange) (bool, error) {
	left, err := s.treeLCA(a.Lo, b.Hi)
	if err != nil {
		return false, err
	}
	right, err := s.treeLCA(a.Hi, b.Lo)
	return err == nil && left == b.Hi && right == a.Hi, err
}

func (s *Scopes) RangeContained(inner, outer ScopeRange) (bool, error) {
	lo, err := s.treeLCA(outer.Lo, inner.Lo)
	if err != nil {
		return false, err
	}
	hi, err := s.treeLCA(inner.Hi, outer.Hi)
	return err == nil && lo == inner.Lo && hi == outer.Hi, err
}

func (s *Scopes) RangeNear(a, b ScopeRange) (int, error) {
	thing, err := s.treeLCA(a.Lo, b.Hi)
	if err != nil {
		return 0, err
	}
	if thing == a.Lo {
		return a.Lo, nil
	}
	return s.treeGCD(thing, a.Hi)
}
