package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// RemovedLabel owns the stripped expression associated with a source label.
// Name belongs to the graph context's symbol table.
type RemovedLabel struct {
	Name     usymbol.Symbol
	Term     *Expr
	Positive bool
}

func (l *RemovedLabel) Close() error {
	if l == nil || l.Term == nil {
		return nil
	}
	if err := l.Term.Close(); err != nil {
		return err
	}
	l.Term = nil
	return nil
}

// RemoveLabels strips labels in child-first order, visiting each shared term
// once. Quantifier annotations remain unchanged; only their bodies are visited.
// The caller owns both the result and each returned label's Term.
// Source: duality_rpfp.cpp, RemoveLabelsRec/RemoveLabels, live FromClauses path.
func (g *RPFP) RemoveLabels(term *Expr) (out *Expr, labels []RemovedLabel, result error) {
	if err := g.open(); err != nil {
		return nil, nil, err
	}
	if err := term.open(); err != nil {
		return nil, nil, err
	}
	if term.value.object.context != g.context {
		return nil, nil, ErrContextMismatch
	}
	m := g.context.manager
	root, err := term.Raw()
	if err != nil {
		return nil, nil, err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			result = errors.Join(result, m.DelRef(&pins[i]))
		}
		if result != nil {
			if out != nil {
				result = errors.Join(result, out.Close())
				out = nil
			}
			for i := range labels {
				result = errors.Join(result, labels[i].Close())
			}
			labels = nil
		}
	}()
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle) (ast.ASTHandle, error) {
		if v, ok := memo[h]; ok {
			return v, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		changed := h
		switch kind {
		case ast.ASTApp:
			var positive bool
			var names []usymbol.Symbol
			labeled, err := m.IsLabel(h, &positive, &names)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if labeled {
				child, err := m.GetArg(h, 0)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed, err = visit(child)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				for _, name := range names {
					value, err := NewExpr(g.context, changed)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					labels = append(labels, RemovedLabel{Name: name, Term: value, Positive: positive})
				}
			} else {
				n, err := m.GetNumArgs(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				args := make([]ast.ASTHandle, n)
				for i := range args {
					child, err := m.GetArg(h, uint32(i))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					args[i], err = visit(child)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
				decl, err := m.GetDecl(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed, err = m.MkApp(decl, args)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			body, err = visit(body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed, err = m.UpdateQuantifierBody(h, body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		pin, err := m.NewRef(changed)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, pin)
		memo[h] = changed
		return changed, nil
	}
	h, err := visit(root)
	if err != nil {
		return nil, labels, err
	}
	out, err = NewExpr(g.context, h)
	return out, labels, err
}
