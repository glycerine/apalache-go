package pdr

// Source: pdr_context.cpp model_node::display and model_search::display.
import (
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (n *ModelNode) WriteDisplay(out io.Writer, indent uint32) error {
	if err := n.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	name, err := n.transformer.headName()
	if err != nil {
		return err
	}
	status := "open"
	if n.closed {
		status = "closed"
	}
	padding := strings.Repeat(" ", int(indent))
	if _, err = fmt.Fprintf(out, "%s%d %s %s\n%s  ", padding, n.level, name, status, padding); err != nil {
		return err
	}
	printer, err := ast.NewSimpleASTPrinterContext(n.transformer.manager, out, out)
	if err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, n.state, 0); err != nil {
		return err
	}
	id, err := n.transformer.manager.GetASTID(n.state)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, " %d\n", id); err != nil {
		return err
	}
	for _, child := range n.children {
		if err = child.WriteDisplay(out, indent+1); err != nil {
			return err
		}
	}
	return nil
}

func (s *ModelSearch) WriteDisplay(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if s.root != nil {
		if err := s.root.WriteDisplay(out, 0); err != nil {
			return err
		}
	}
	count, err := s.GoalCount()
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "goals %d\n", count); err != nil {
		return err
	}
	if s.goal != nil {
		n := s.goal
		for {
			if err = n.WriteDisplay(out, 1); err != nil {
				return err
			}
			n = n.next
			if n == s.goal {
				break
			}
		}
	}
	return nil
}
