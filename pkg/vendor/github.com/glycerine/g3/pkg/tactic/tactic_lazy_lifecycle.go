package tactic

// Source: src/tactic/tactic.cpp lazy_tactic lifecycle forwarding.

import "github.com/glycerine/g3/pkg/ast"

func (t *lazyTactic) CollectStatistics() map[string]float64 {
	if t.open() != nil || t.child == nil {
		return map[string]float64{}
	}
	return t.child.CollectStatistics()
}
func (t *lazyTactic) ResetStatistics() error {
	if e := t.open(); e != nil {
		return e
	}
	if t.child != nil {
		return t.child.ResetStatistics()
	}
	return nil
}
func (t *lazyTactic) Cleanup() error {
	if e := t.open(); e != nil {
		return e
	}
	if t.child != nil {
		return t.child.Cleanup()
	}
	return nil
}
func (t *lazyTactic) Reset() error {
	if e := t.open(); e != nil {
		return e
	}
	if t.child != nil {
		return t.child.Reset()
	}
	return nil
}
func (t *lazyTactic) Translate(m *ast.ASTManager) (Tactic, error) {
	if e := t.ensure(); e != nil {
		return nil, e
	}
	return t.child.Translate(m)
}
