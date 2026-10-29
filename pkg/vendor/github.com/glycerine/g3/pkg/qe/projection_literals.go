package qe

// Source: src/qe/qe_mbp.cpp extract_literals/extract_bools/pick_equality.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (c *projectionPreprocessCall) negate(h ast.ASTHandle) (ast.ASTHandle, error) {
	return c.hold(c.owner.manager.MkNotSimplified(h))
}
func (c *projectionPreprocessCall) pickEquality(distinct projectionApplication) (ast.ASTHandle, error) {
	out, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = pickProjectionEquality(c.owner.manager, c.evaluator, distinct.args, &c.refs, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.RefGet(out)
}

// The actual caller's mutable reference vector owns every evaluated value
// through stopped cleanup. Both MBP preprocessing and datatype solving reach
// this concrete project_plugin::pick_equality implementation.
func pickProjectionEquality(m *ast.ASTManager, evaluator *model.ModelEvaluator, args []ast.ASTHandle, roots *[]ast.ASTRef, out ast.ASTRef) error {
	values := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, h := range args {
		r, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		*roots = append(*roots, r)
		if err = evaluator.Evaluate(h, r); err != nil {
			return err
		}
		v, err := m.RefGet(r)
		if err != nil {
			return err
		}
		if previous, ok := values[v]; ok {
			eq, err := m.MkEq(h, previous)
			if err != nil {
				return err
			}
			return m.SetRef(out, eq)
		}
		values[v] = h
	}
	return ast.ErrASTDomain
}

func (p *projectionPreprocessor) extractNestedBooleans(c *projectionPreprocessCall, forms *[]ast.ASTRef, formula ast.ASTHandle) (found bool, err error) {
	m := p.manager
	if err = p.emit("extract_bools", 218, func(w io.Writer) error {
		_, _ = io.WriteString(w, "extract bools: ")
		if e := writeExpression(m, p.config, w, formula); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return false, err
	}
	sub, err := c.replacement()
	if err != nil {
		return false, err
	}
	visited := make(map[ast.ASTHandle]bool)
	a, err := p.application(formula)
	if err != nil {
		return false, err
	}
	todo := append([]ast.ASTHandle(nil), a.args...)
	for len(todo) != 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if visited[h] {
			continue
		}
		visited[h] = true
		boolean, e := p.boolean(h)
		if e != nil {
			return found, e
		}
		if boolean && !m.IsTrue(h) && !m.IsFalse(h) {
			v, e := c.eval(h)
			if e != nil {
				return found, e
			}
			if e = p.emit("extract_bools", 235, func(w io.Writer) error {
				_, _ = io.WriteString(w, "found: ")
				if e := writeExpression(m, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); e != nil {
				return found, e
			}
			if !m.IsTrue(v) && !m.IsFalse(v) {
				return found, ast.ErrASTDomain
			}
			if !p.boolVisited[h] {
				lit := h
				if m.IsFalse(v) {
					lit, e = c.negate(h)
					if e != nil {
						return found, e
					}
				}
				if e = p.appendRef(forms, lit); e != nil {
					return found, e
				}
			}
			if e = sub.Insert(h, v); e != nil {
				return found, e
			}
			p.boolVisited[h] = true
			found = true
		} else {
			kind, e := m.GetKind(h)
			if e != nil {
				return found, e
			}
			if kind == ast.ASTApp {
				app, e := p.application(h)
				if e != nil {
					return found, e
				}
				todo = append(todo, app.args...)
			} else {
				if e = p.emit("extract_bools", 248, func(w io.Writer) error {
					_, _ = io.WriteString(w, "expression not handled ")
					if e := writeExpression(m, p.config, w, h); e != nil {
						return e
					}
					_, _ = io.WriteString(w, "\n")
					return nil
				}); e != nil {
					return found, e
				}
			}
		}
	}
	if found {
		out, e := c.ref(ast.ASTHandle{})
		if e != nil {
			return found, e
		}
		if e = sub.Apply(formula, out); e != nil {
			return found, e
		}
		h, e := m.RefGet(out)
		if e != nil {
			return found, e
		}
		v, e := c.eval(h)
		if e != nil {
			return found, e
		}
		if !m.IsTrue(v) && !m.IsFalse(v) {
			return found, ast.ErrASTDomain
		}
		if m.IsFalse(v) {
			h, e = c.negate(h)
			if e != nil {
				return found, e
			}
		}
		if e = p.appendRef(forms, h); e != nil {
			return found, e
		}
	}
	return found, nil
}

func (p *projectionPreprocessor) extractLiterals(md *model.ModelCore, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if forms == nil {
		return ast.ErrASTHandle
	}
	if err = c.startEvaluator(md); err != nil {
		return err
	}
	if err = p.emit("extract_literals", 310, func(w io.Writer) error {
		if e := p.displayForms(w, *forms); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	m := p.manager
	for i := 0; i < len(*forms); i++ {
		formula, e := m.RefGet((*forms)[i])
		if e != nil {
			return e
		}
		boolean, e := p.boolean(formula)
		if e != nil {
			return e
		}
		if !boolean {
			return ast.ErrASTDomain
		}
		// Retain the actual source local expression while replacing/erasing
		// its vector slot, so borrowed child handles remain live.
		if _, e = c.ref(formula); e != nil {
			return e
		}
		a, e := p.application(formula)
		if e != nil {
			return e
		}
		negative := a.basic(ast.ASTOpNot) && len(a.args) == 1
		inner := formula
		na := projectionApplication{family: ast.NullFamilyID, op: -1}
		if negative {
			inner = a.args[0]
			na, e = p.application(inner)
			if e != nil {
				return e
			}
		}
		switch {
		case negative && na.basic(ast.ASTOpDistinct):
			h, e := c.pickEquality(na)
			if e != nil {
				return e
			}
			if e = m.SetRef((*forms)[i], h); e != nil {
				return e
			}
			i--
		case a.basic(ast.ASTOpOr):
			for _, arg := range a.args {
				v, e := c.eval(arg)
				if e != nil {
					return e
				}
				if m.IsTrue(v) {
					if e = m.SetRef((*forms)[i], arg); e != nil {
						return e
					}
					i--
					break
				}
			}
		case a.basic(ast.ASTOpAnd):
			for _, arg := range a.args {
				if e = p.appendRef(forms, arg); e != nil {
					return e
				}
			}
			if e = p.erase(forms, &i); e != nil {
				return e
			}
		case (a.basic(ast.ASTOpIff) && len(a.args) == 2) || (negative && na.basic(ast.ASTOpXor) && len(na.args) == 2):
			args := a.args
			if negative {
				args = na.args
			}
			first, second := args[0], args[1]
			v, e := c.eval(first)
			if e != nil {
				return e
			}
			if m.IsFalse(v) {
				first, e = c.negate(first)
				if e != nil {
					return e
				}
				second, e = c.negate(second)
				if e != nil {
					return e
				}
			}
			if e = m.SetRef((*forms)[i], first); e != nil {
				return e
			}
			if e = p.push(forms, second); e != nil {
				return e
			}
			i--
		case a.basic(ast.ASTOpImplies) && len(a.args) == 2:
			v, e := c.eval(a.args[1])
			if e != nil {
				return e
			}
			h := a.args[1]
			if !m.IsTrue(v) {
				h, e = c.negate(a.args[0])
				if e != nil {
					return e
				}
			}
			if e = m.SetRef((*forms)[i], h); e != nil {
				return e
			}
			i--
		case a.basic(ast.ASTOpIte) && len(a.args) == 3:
			v, e := c.eval(a.args[0])
			if e != nil {
				return e
			}
			condition, branch := a.args[0], a.args[1]
			if !m.IsTrue(v) {
				condition, e = c.negate(condition)
				if e != nil {
					return e
				}
				branch = a.args[2]
			}
			if e = p.push(forms, condition); e != nil {
				return e
			}
			if e = p.push(forms, branch); e != nil {
				return e
			}
			if e = p.erase(forms, &i); e != nil {
				return e
			}
		case negative && na.basic(ast.ASTOpNot) && len(na.args) == 1:
			if e = p.push(forms, na.args[0]); e != nil {
				return e
			}
			if e = p.erase(forms, &i); e != nil {
				return e
			}
		case negative && na.basic(ast.ASTOpAnd):
			for _, arg := range na.args {
				v, e := c.eval(arg)
				if e != nil {
					return e
				}
				if m.IsFalse(v) {
					h, e := c.negate(arg)
					if e != nil {
						return e
					}
					if e = m.SetRef((*forms)[i], h); e != nil {
						return e
					}
					i--
					break
				}
			}
		case negative && na.basic(ast.ASTOpOr):
			for _, arg := range na.args {
				h, e := c.negate(arg)
				if e != nil {
					return e
				}
				if e = p.push(forms, h); e != nil {
					return e
				}
			}
			if e = p.erase(forms, &i); e != nil {
				return e
			}
		case (negative && na.basic(ast.ASTOpIff) && len(na.args) == 2) || (a.basic(ast.ASTOpXor) && len(a.args) == 2):
			args := a.args
			if negative {
				args = na.args
			}
			first, second := args[0], args[1]
			v, e := c.eval(first)
			if e != nil {
				return e
			}
			if m.IsTrue(v) {
				second, e = c.negate(second)
				if e != nil {
					return e
				}
			} else {
				first, e = c.negate(first)
				if e != nil {
					return e
				}
			}
			if e = p.push(forms, first); e != nil {
				return e
			}
			if e = p.push(forms, second); e != nil {
				return e
			}
			if e = p.erase(forms, &i); e != nil {
				return e
			}
		case negative && na.basic(ast.ASTOpImplies) && len(na.args) == 2:
			if e = p.push(forms, na.args[0]); e != nil {
				return e
			}
			h, e := c.negate(na.args[1])
			if e != nil {
				return e
			}
			if e = p.push(forms, h); e != nil {
				return e
			}
			if e = p.erase(forms, &i); e != nil {
				return e
			}
		case negative && na.basic(ast.ASTOpIte) && len(na.args) == 3:
			v, e := c.eval(na.args[0])
			if e != nil {
				return e
			}
			condition, branch := na.args[0], na.args[1]
			if !m.IsTrue(v) {
				condition, e = c.negate(condition)
				if e != nil {
					return e
				}
				branch = na.args[2]
			}
			if e = p.push(forms, condition); e != nil {
				return e
			}
			branch, e = c.negate(branch)
			if e != nil {
				return e
			}
			if e = p.push(forms, branch); e != nil {
				return e
			}
			if e = p.erase(forms, &i); e != nil {
				return e
			}
		default:
			h := formula
			if negative {
				h = inner
			}
			found, e := p.extractNestedBooleans(c, forms, h)
			if e != nil {
				return e
			}
			if found {
				if e = p.erase(forms, &i); e != nil {
					return e
				}
			}
		}
	}
	if err = p.emit("extract_literals", 425, func(w io.Writer) error {
		if e := p.displayForms(w, *forms); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	clear(p.boolVisited)
	return nil
}
