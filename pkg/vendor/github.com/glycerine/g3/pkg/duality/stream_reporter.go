package duality

// Source: duality_solver.cpp, StreamReporter.
import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"io"
)

// StreamReporter borrows its formatter services and destination. Its event
// sequence and depth belong to one search, as in the source reporter instance.
type StreamReporter struct {
	Format ConjectureReporter
	event  int
	depth  int
}

func (r *StreamReporter) prefix() error {
	if r.Format.Output == nil {
		return ErrWrapperOwner
	}
	_, err := fmt.Fprintf(r.Format.Output, "[%d]", r.event)
	r.event++
	return err
}
func (r *StreamReporter) nodeName(node *Node) (string, error) {
	if node == nil || node.closed || node.Owner == nil {
		return "", ErrWrapperOwner
	}
	m := node.Owner.context.manager
	decl, err := m.RefGet(node.Name)
	if err != nil {
		return "", err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return "", err
	}
	return m.Symbols.String(name)
}
func (r *StreamReporter) Extend(node *Node) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(node)
	if err != nil {
		return err
	}
	if node.Outgoing == nil {
		return ErrWrapperOwner
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "node %d: %s", node.Number, name); err != nil {
		return err
	}
	for _, child := range node.Outgoing.Children {
		if _, err := fmt.Fprintf(r.Format.Output, " %d", child.Number); err != nil {
			return err
		}
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}
func (r *StreamReporter) Bound(node *Node) error {
	if r == nil {
		return nil
	}
	if _, err := r.nodeName(node); err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(r.Format.Output, "check %d\n", node.Number)
	return err
}
func (r *StreamReporter) summarize(update *Transformer) error {
	if err := update.open(); err != nil {
		return err
	}
	m := update.context.manager
	root, err := m.RefGet(update.formula)
	if err != nil {
		return err
	}
	seen := make(map[ast.ASTHandle]bool)
	ops := 0
	var literals []ast.ASTHandle
	var visit func(ast.ASTHandle) error
	visit = func(h ast.ASTHandle) error {
		if seen[h] {
			return nil
		}
		seen[h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return err
		}
		if kind == ast.ASTApp {
			family, err := m.GetFamilyID(h)
			if err != nil {
				return err
			}
			op, err := m.GetDeclKind(h)
			if err != nil {
				return err
			}
			if family == m.BasicFamilyID() && (op == ast.ASTOpAnd || op == ast.ASTOpOr || op == ast.ASTOpNot || op == ast.ASTOpImplies || op == ast.ASTOpIff) {
				ops++
				n, err := m.GetNumArgs(h)
				if err != nil {
					return err
				}
				for i := uint32(0); i < n; i++ {
					a, err := m.GetArg(h, i)
					if err != nil {
						return err
					}
					if err := visit(a); err != nil {
						return err
					}
				}
				return nil
			}
		}
		literals = append(literals, h)
		return nil
	}
	if err := visit(root); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "%d: ", ops); err != nil {
		return err
	}
	for i, h := range literals {
		if i > 0 {
			if _, err := io.WriteString(r.Format.Output, ", "); err != nil {
				return err
			}
		}
		if err := r.Format.writeTerm(m, h, false); err != nil {
			return err
		}
	}
	return nil
}
func (r *StreamReporter) Update(node *Node, update *Transformer, eager bool) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(node)
	if err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "update %d %s: ", node.Number, name); err != nil {
		return err
	}
	if err := r.summarize(update); err != nil {
		return err
	}
	if r.depth > 0 {
		if _, err := fmt.Fprintf(r.Format.Output, " (depth=%d)", r.depth); err != nil {
			return err
		}
	}
	if eager {
		if _, err := io.WriteString(r.Format.Output, " (eager)"); err != nil {
			return err
		}
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}

func (r *StreamReporter) Expand(edge *Edge) error {
	if r == nil {
		return nil
	}
	if edge == nil || edge.closed || edge.Parent == nil || edge.Parent.Map == nil {
		return ErrWrapperOwner
	}
	node := edge.Parent
	name, err := r.nodeName(node)
	if err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "expand %d %s", node.Map.Number, name); err != nil {
		return err
	}
	if r.depth > 0 {
		if _, err := fmt.Fprintf(r.Format.Output, " (depth=%d)", r.depth); err != nil {
			return err
		}
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}
func (r *StreamReporter) Conjecture(node *Node, formula *Transformer) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(node)
	if err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "conjecture %d %s: ", node.Number, name); err != nil {
		return err
	}
	if err := r.summarize(formula); err != nil {
		return err
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}
func (r *StreamReporter) UpdateUnderapprox(node *Node, formula *Transformer) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(node)
	if err != nil {
		return err
	}
	if err := formula.open(); err != nil {
		return err
	}
	h, err := formula.context.manager.RefGet(formula.formula)
	if err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "underapprox %d %s: ", node.Number, name); err != nil {
		return err
	}
	if err := r.Format.writeTerm(formula.context.manager, h, false); err != nil {
		return err
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}
func (r *StreamReporter) Depth(depth int) {
	if r != nil {
		r.depth = depth
	}
}

func (r *StreamReporter) InductionFailure(edge *Edge, children []*Node) error {
	if r == nil {
		return nil
	}
	if edge == nil || edge.closed {
		return ErrWrapperOwner
	}
	name, err := r.nodeName(edge.Parent)
	if err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "induction failure: %s, children =", name); err != nil {
		return err
	}
	for _, child := range children {
		if child == nil || child.closed {
			return ErrWrapperOwner
		}
		if _, err := fmt.Fprintf(r.Format.Output, " %d", child.Number); err != nil {
			return err
		}
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}
func (r *StreamReporter) Reject(edge *Edge, children []*Node) error {
	if r == nil {
		return nil
	}
	if edge == nil || edge.closed {
		return ErrWrapperOwner
	}
	name, err := r.nodeName(edge.Parent)
	if err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "reject %d %s: ", edge.Parent.Number, name); err != nil {
		return err
	}
	for _, child := range children {
		if child == nil || child.closed {
			return ErrWrapperOwner
		}
		if _, err := fmt.Fprintf(r.Format.Output, " %d", child.Number); err != nil {
			return err
		}
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}
func (r *StreamReporter) Message(message string) error {
	if r == nil {
		return nil
	}
	if err := r.prefix(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(r.Format.Output, "msg %s\n", message)
	return err
}

func (r *StreamReporter) AddCover(covered *Node, covering []*Node) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(covered)
	if err != nil {
		return err
	}
	if err := r.prefix(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Format.Output, "cover %s: %d by ", name, covered.Number); err != nil {
		return err
	}
	for _, node := range covering {
		if node == nil || node.closed {
			return ErrWrapperOwner
		}
		if _, err := fmt.Fprintf(r.Format.Output, "%d ", node.Number); err != nil {
			return err
		}
	}
	_, err = io.WriteString(r.Format.Output, "\n")
	return err
}
func (r *StreamReporter) RemoveCover(covered, covering *Node) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(covered)
	if err != nil {
		return err
	}
	if covering == nil || covering.closed {
		return ErrWrapperOwner
	}
	if err := r.prefix(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(r.Format.Output, "uncover %s: %d by %d\n", name, covered.Number, covering.Number)
	return err
}
func (r *StreamReporter) Forcing(covered, covering *Node) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(covered)
	if err != nil {
		return err
	}
	if covering == nil || covering.closed {
		return ErrWrapperOwner
	}
	if err := r.prefix(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(r.Format.Output, "forcing %s: %d by %d\n", name, covered.Number, covering.Number)
	return err
}
func (r *StreamReporter) Dominates(node, other *Node) error {
	if r == nil {
		return nil
	}
	name, err := r.nodeName(node)
	if err != nil {
		return err
	}
	if other == nil || other.closed {
		return ErrWrapperOwner
	}
	if err := r.prefix(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(r.Format.Output, "dominates %s: %d > %d\n", name, node.Number, other.Number)
	return err
}
