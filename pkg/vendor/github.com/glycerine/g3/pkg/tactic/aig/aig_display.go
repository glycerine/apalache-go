package aig

// Source: src/tactic/aig/aig.cpp:1548-1591,1739-1741.

import (
	"bytes"
	"io"
	"strconv"
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

func (m *Manager) writeAIGSMT2Ref(out io.Writer, l Literal) error {
	if l.inverted {
		if _, err := io.WriteString(out, "(not "); err != nil {
			return err
		}
	}
	if !l.node.and {
		h, err := m.ast.RefGet(l.node.variable)
		if err != nil {
			return err
		}
		body, err := m.smt2BooleanText(h)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, body); err != nil {
			return err
		}
	} else if _, err := io.WriteString(out, "aig"+strconv.FormatUint(uint64(l.node.id-firstNodeID), 10)); err != nil {
		return err
	}
	if l.inverted {
		_, err := io.WriteString(out, ")")
		return err
	}
	return nil
}

// DisplaySMT2 emits dependency-ordered definitions for the AIG and then asserts
// the requested root, matching aig_manager::display_smt2.
func (m *Manager) DisplaySMT2(out io.Writer, r *Ref) error {
	if out == nil {
		return ErrOwner
	}
	root, err := r.literal(m)
	if err != nil {
		return err
	}
	todo := []Literal{root}
	marked := make(map[*node]bool)
	for len(todo) != 0 {
		top := len(todo) - 1
		current := todo[top]
		if marked[current.node] {
			todo = todo[:top]
			continue
		}
		if !current.node.and {
			marked[current.node] = true
			todo = todo[:top]
			continue
		}
		visited := true
		for _, child := range current.node.children {
			if !marked[child.node] {
				todo = append(todo, child)
				visited = false
			}
		}
		if !visited {
			continue
		}
		if _, err = io.WriteString(out, "(define-fun aig"+strconv.FormatUint(uint64(current.node.id-firstNodeID), 10)+" () Bool (and"); err != nil {
			return err
		}
		for _, child := range current.node.children {
			if _, err = io.WriteString(out, " "); err != nil {
				return err
			}
			if err = m.writeAIGSMT2Ref(out, child); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "))\n"); err != nil {
			return err
		}
		marked[current.node] = true
		todo = todo[:top]
	}
	if _, err = io.WriteString(out, "(assert "); err != nil {
		return err
	}
	if err = m.writeAIGSMT2Ref(out, root); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")\n")
	return err
}
