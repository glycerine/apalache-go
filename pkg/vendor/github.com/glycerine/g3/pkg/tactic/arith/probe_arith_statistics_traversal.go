package arith

// Source: src/ast/for_each_expr.h:27-105, MarkAll goal specialization.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func statisticApplications(g *tactic.Goal) ([]ast.ASTHandle, error) {
	m := g.Manager()
	seen := map[ast.ASTHandle]bool{}
	var out []ast.ASTHandle
	var stack []ast.ASTHandle
	n, e := g.Size()
	if e != nil {
		return nil, e
	}
	for i := uint32(0); i < n; i++ {
		h, e := g.Form(i)
		if e != nil {
			return nil, e
		}
		stack = append(stack, h)
	}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		k, e := m.GetKind(h)
		if e != nil {
			return nil, e
		}
		if k != ast.ASTApp {
			continue
		}
		out = append(out, h)
		n, e := m.GetNumArgs(h)
		if e != nil {
			return nil, e
		}
		for i := uint32(0); i < n; i++ {
			a, e := m.GetArg(h, i)
			if e != nil {
				return nil, e
			}
			stack = append(stack, a)
		}
	}
	return out, nil
}
