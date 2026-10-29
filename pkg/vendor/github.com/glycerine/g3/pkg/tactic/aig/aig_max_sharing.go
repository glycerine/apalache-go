package aig

// Source: src/tactic/aig/aig.cpp:1030-1297.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type sharingFrame struct {
	node *node
	next uint8
}
type sharingPass struct {
	manager *Manager
	frames  []sharingFrame
	results []*Ref // nil denotes unchanged source node
	cache   map[*node]*Ref
	saved   []*Ref // source m_saved old-node pins
}

func (p *sharingPass) cleanup() error {
	var err error
	for i := len(p.results) - 1; i >= 0; i-- {
		if p.results[i] != nil {
			err = errors.Join(err, p.results[i].Close())
		}
	}
	for _, r := range p.cache {
		err = errors.Join(err, r.Close())
	}
	for i := len(p.saved) - 1; i >= 0; i-- {
		err = errors.Join(err, p.saved[i].Close())
	}
	p.results = nil
	p.cache = nil
	p.saved = nil
	p.frames = nil
	return err
}

func (p *sharingPass) cached(n *node) (bool, error) {
	if n.refs <= 1 {
		return false, nil
	}
	if r := p.cache[n]; r != nil {
		copy, err := r.Clone()
		if err != nil {
			return false, err
		}
		p.results = append(p.results, copy)
		return true, nil
	}
	return false, nil
}
func (p *sharingPass) visit(n *node) error {
	if !n.and {
		p.results = append(p.results, nil)
		return nil
	}
	if found, err := p.cached(n); found || err != nil {
		return err
	}
	p.frames = append(p.frames, sharingFrame{node: n})
	return nil
}
func (p *sharingPass) saveResult(old *node, result Literal) error {
	m := p.manager
	if old.refs > 1 {
		oldPin, err := m.newRef(Literal{node: old})
		if err != nil {
			return err
		}
		newPin, err := m.newRef(result)
		if err != nil {
			return errors.Join(err, oldPin.Close())
		}
		p.saved = append(p.saved, oldPin)
		p.cache[old] = newPin
	}
	if old != result.node {
		owned, err := m.newRef(result)
		if err != nil {
			return err
		}
		p.results = append(p.results, owned)
		return nil
	}
	if result.inverted {
		return ast.ErrASTDomain
	}
	p.results = append(p.results, nil)
	return nil
}
func (p *sharingPass) pop2() error {
	n := len(p.results)
	if n < 2 {
		return ast.ErrASTDomain
	}
	right, left := p.results[n-1], p.results[n-2]
	p.results = p.results[:n-2]
	var err error
	if right != nil {
		err = errors.Join(err, right.Close())
	}
	if left != nil {
		err = errors.Join(err, left.Close())
	}
	return err
}

func (p *sharingPass) tryLeft(old *node, n Literal) (bool, error) {
	m := p.manager
	l := n.node.children[0]
	a, b := l.node.children[0], l.node.children[1]
	c := n.node.children[1]
	for i, row := range [][3]Literal{{b, c, a}, {a, c, b}} {
		m.traceSharingTry(a, b, c, true, i == 1)
		inner, err := m.mkAnd(row[0], row[1])
		if err != nil {
			return false, err
		}
		pin, err := m.newRef(inner)
		if err != nil {
			return false, err
		}
		if inner.node.refs > 1 {
			outer, buildErr := m.mkAnd(row[2], inner)
			if buildErr == nil {
				if n.inverted {
					outer = invert(outer)
				}
				buildErr = p.saveResult(old, outer)
			}
			closeErr := pin.Close()
			if buildErr == nil && closeErr == nil {
				buildErr = m.traceSharingImproved(old, outer, true, i == 1)
			}
			return true, errors.Join(buildErr, closeErr)
		}
		if err = pin.Close(); err != nil {
			return false, err
		}
	}
	return false, nil
}
func (p *sharingPass) tryRight(old *node, n Literal) (bool, error) {
	m := p.manager
	r := n.node.children[1]
	a := n.node.children[0]
	b, c := r.node.children[0], r.node.children[1]
	for i, row := range [][3]Literal{{a, b, c}, {a, c, b}} {
		if i == 0 {
			m.traceSharingTry(a, b, c, false, false)
		}
		inner, err := m.mkAnd(row[0], row[1])
		if err != nil {
			return false, err
		}
		if i == 1 {
			m.traceSharingTry(a, b, c, false, true)
		}
		pin, err := m.newRef(inner)
		if err != nil {
			return false, err
		}
		if inner.node.refs > 1 {
			outer, buildErr := m.mkAnd(inner, row[2])
			if buildErr == nil {
				if n.inverted {
					outer = invert(outer)
				}
				buildErr = p.saveResult(old, outer)
			}
			closeErr := pin.Close()
			if buildErr == nil && closeErr == nil {
				buildErr = m.traceSharingImproved(old, outer, false, i == 1)
			}
			return true, errors.Join(buildErr, closeErr)
		}
		if err = pin.Close(); err != nil {
			return false, err
		}
	}
	return false, nil
}
func (p *sharingPass) improveCore(old *node, n Literal) error {
	if n.node.and {
		l := n.node.children[0]
		if !l.inverted && l.node.refs == 1 && l.node.and {
			if changed, err := p.tryLeft(old, n); changed || err != nil {
				return err
			}
		}
		r := n.node.children[1]
		if !r.inverted && r.node.refs == 1 && r.node.and {
			if changed, err := p.tryRight(old, n); changed || err != nil {
				return err
			}
		}
	}
	return p.saveResult(old, n)
}
func (p *sharingPass) improve(n *node) error {
	sz := len(p.results)
	if sz < 2 {
		return ast.ErrASTDomain
	}
	left, right := p.results[sz-2], p.results[sz-1]
	if left == nil && right == nil {
		if err := p.pop2(); err != nil {
			return err
		}
		return p.improveCore(n, Literal{node: n})
	}
	l, r := n.children[0], n.children[1]
	if left != nil {
		l = left.lit
		if n.children[0].inverted {
			l = invert(l)
		}
	}
	if right != nil {
		r = right.lit
		if n.children[1].inverted {
			r = invert(r)
		}
	}
	newNode, err := p.manager.mkAnd(l, r)
	if err != nil {
		return err
	}
	pin, err := p.manager.newRef(newNode)
	if err != nil {
		return err
	}
	err = p.pop2()
	if err == nil {
		err = p.improveCore(n, newNode)
	}
	return errors.Join(err, pin.Close())
}

func (p *sharingPass) process(n *node) error {
	if err := p.visit(n); err != nil {
		return err
	}
	for len(p.frames) != 0 {
		last := len(p.frames) - 1
		fr := &p.frames[last]
		p.manager.traceSharingProcessing(fr.node, fr.next)
		if fr.next == 0 {
			fr.next = 1
			child := fr.node.children[0].node
			if err := p.visit(child); err != nil {
				return err
			}
			if len(p.frames) != last+1 {
				continue
			}
		}
		if fr.next == 1 {
			fr.next = 2
			child := fr.node.children[1].node
			if err := p.visit(child); err != nil {
				return err
			}
			if len(p.frames) != last+1 {
				continue
			}
		}
		found, err := p.cached(fr.node)
		if err != nil {
			return err
		}
		if !found {
			if err = p.improve(fr.node); err != nil {
				return err
			}
		}
		p.frames = p.frames[:last]
	}
	return nil
}

// MaxSharing follows the source's postorder associative reassociation pass.
// The returned reference owns one count; the input remains owned by the caller.
func (m *Manager) MaxSharing(input *Ref) (out *Ref, err error) {
	l, e := input.literal(m)
	if e != nil {
		return nil, e
	}
	p := sharingPass{manager: m, cache: make(map[*node]*Ref)}
	defer func() {
		err = errors.Join(err, p.cleanup())
		if err != nil && out != nil {
			err = errors.Join(err, out.Close())
			out = nil
		}
	}()
	if err = p.process(l.node); err != nil {
		return nil, err
	}
	if len(p.results) != 1 {
		return nil, ast.ErrASTDomain
	}
	result := p.results[0]
	m.traceSharingNull(result == nil)
	p.results = nil
	if err = p.cleanup(); err != nil {
		if result != nil {
			err = errors.Join(err, result.Close())
		}
		return nil, err
	}
	if result == nil {
		out, err = input.Clone()
	} else if l.inverted {
		out, err = result.Not()
		err = errors.Join(err, result.Close())
	} else {
		out = result
	}
	if err != nil {
		return out, err
	}
	err = m.traceSharingResult(out.lit)
	return out, err
}
