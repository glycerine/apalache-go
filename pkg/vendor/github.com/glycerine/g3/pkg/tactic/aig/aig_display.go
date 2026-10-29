package aig

// Source: src/tactic/aig/aig.cpp:1548-1591,1739-1741.

import (
	"bytes"
	"io"
)

func aigRefText(l Literal) string {
	var out bytes.Buffer
	writeAIGRef(&out, l)
	return out.String()
}

func (m *Manager) writeAIGDisplay(out io.Writer, root Literal) error {
	if root.node == nil {
		return ErrOwner
	}
	if _, err := io.WriteString(out, aigRefText(root)+"\n"); err != nil {
		return err
	}
	queue := []*node{root.node}
	marked := make(map[*node]bool)
	for head := 0; head < len(queue); head++ {
		n := queue[head]
		if _, err := io.WriteString(out, aigRefText(Literal{node: n})+": "); err != nil {
			return err
		}
		if !n.and {
			h, err := m.ast.RefGet(n.variable)
			if err != nil {
				return err
			}
			body, err := m.smt2BooleanText(h)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(out, body+"\n"); err != nil {
				return err
			}
			continue
		}
		if _, err := io.WriteString(out, aigRefText(n.children[0])+" "+aigRefText(n.children[1])+"\n"); err != nil {
			return err
		}
		for _, child := range n.children {
			if !marked[child.node] {
				marked[child.node] = true
				queue = append(queue, child.node)
			}
		}
	}
	return nil
}

// Display prints the source breadth-first reference and node listing.
func (m *Manager) Display(out io.Writer, r *Ref) error {
	if out == nil {
		return ErrOwner
	}
	l, err := r.literal(m)
	if err != nil {
		return err
	}
	return m.writeAIGDisplay(out, l)
}
