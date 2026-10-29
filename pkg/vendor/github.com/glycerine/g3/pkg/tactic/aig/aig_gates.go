package aig

// Source: src/tactic/aig/aig.cpp:1341-1472,1699-1722.

import (
	"errors"
	"sort"
)

type gateTemps struct{ refs []*Ref }

func (t *gateTemps) keep(r *Ref, err error) (*Ref, error) {
	if err == nil {
		t.refs = append(t.refs, r)
	}
	return r, err
}

func (m *Manager) composeGate(build func(*gateTemps) (*Ref, error)) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	t := &gateTemps{}
	result, err := build(t)
	for i := len(t.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, t.refs[i].Close())
	}
	if err != nil && result != nil {
		err = errors.Join(err, result.Close())
		result = nil
	}
	return result, err
}

// Or follows the source's De Morgan gate construction.
func (m *Manager) Or(left, right *Ref) (*Ref, error) {
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		nr, err := t.keep(right.Not())
		if err != nil {
			return nil, err
		}
		conj, err := t.keep(m.And(nl, nr))
		if err != nil {
			return nil, err
		}
		return conj.Not()
	})
}

// Ite selects the source's default or alternate gate encoding.
func (m *Manager) Ite(condition, then, otherwise *Ref) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if !m.defaultGateEncoding {
		return m.composeGate(func(t *gateTemps) (*Ref, error) {
			first, err := t.keep(m.And(condition, then))
			if err != nil {
				return nil, err
			}
			nc, err := t.keep(condition.Not())
			if err != nil {
				return nil, err
			}
			second, err := t.keep(m.And(nc, otherwise))
			if err != nil {
				return nil, err
			}
			return m.Or(first, second)
		})
	}
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nc, err := t.keep(condition.Not())
		if err != nil {
			return nil, err
		}
		first, err := t.keep(m.Or(nc, then))
		if err != nil {
			return nil, err
		}
		second, err := t.keep(m.Or(condition, otherwise))
		if err != nil {
			return nil, err
		}
		return m.And(first, second)
	})
}

// Iff selects the source's default or alternate gate encoding.
func (m *Manager) Iff(left, right *Ref) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if !m.defaultGateEncoding {
		return m.composeGate(func(t *gateTemps) (*Ref, error) {
			first, err := t.keep(m.And(left, right))
			if err != nil {
				return nil, err
			}
			nl, err := t.keep(left.Not())
			if err != nil {
				return nil, err
			}
			nr, err := t.keep(right.Not())
			if err != nil {
				return nil, err
			}
			second, err := t.keep(m.And(nl, nr))
			if err != nil {
				return nil, err
			}
			return m.Or(first, second)
		})
	}
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		nr, err := t.keep(right.Not())
		if err != nil {
			return nil, err
		}
		first, err := t.keep(m.Or(nl, right))
		if err != nil {
			return nil, err
		}
		second, err := t.keep(m.Or(left, nr))
		if err != nil {
			return nil, err
		}
		return m.And(first, second)
	})
}

func (m *Manager) Xor(left, right *Ref) (*Ref, error) {
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		return m.Iff(nl, right)
	})
}

func (m *Manager) Implies(left, right *Ref) (*Ref, error) {
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		return m.Or(nl, right)
	})
}

func (m *Manager) sortedArgs(args []*Ref) ([]*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	ordered := append([]*Ref(nil), args...)
	for _, arg := range ordered {
		if _, err := arg.literal(m); err != nil {
			return nil, err
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i].lit, ordered[j].lit
		if a.node.id != b.node.id {
			return a.node.id < b.node.id
		}
		return a.inverted && !b.inverted
	})
	return ordered, nil
}

// AndArgs follows the source's sorted left fold for three or more operands.
func (m *Manager) AndArgs(args []*Ref) (*Ref, error) {
	ordered, err := m.sortedArgs(args)
	if err != nil {
		return nil, err
	}
	switch len(ordered) {
	case 0:
		return m.True()
	case 1:
		return ordered[0].Clone()
	}
	acc, err := m.And(ordered[0], ordered[1])
	if err != nil {
		return nil, err
	}
	for _, next := range ordered[2:] {
		replacement, e := m.And(acc, next)
		closeErr := acc.Close()
		if e != nil || closeErr != nil {
			if replacement != nil {
				closeErr = errors.Join(closeErr, replacement.Close())
			}
			return nil, errors.Join(e, closeErr)
		}
		acc = replacement
	}
	return acc, nil
}

// OrArgs follows the source's sorted left fold for three or more operands.
func (m *Manager) OrArgs(args []*Ref) (*Ref, error) {
	ordered, err := m.sortedArgs(args)
	if err != nil {
		return nil, err
	}
	switch len(ordered) {
	case 0:
		return m.False()
	case 1:
		return ordered[0].Clone()
	}
	acc, err := m.Or(ordered[0], ordered[1])
	if err != nil {
		return nil, err
	}
	for _, next := range ordered[2:] {
		replacement, e := m.Or(acc, next)
		closeErr := acc.Close()
		if e != nil || closeErr != nil {
			if replacement != nil {
				closeErr = errors.Join(closeErr, replacement.Close())
			}
			return nil, errors.Join(e, closeErr)
		}
		acc = replacement
	}
	return acc, nil
}
