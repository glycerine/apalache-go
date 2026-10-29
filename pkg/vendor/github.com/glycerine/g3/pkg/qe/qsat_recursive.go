package qe

// Source: src/qe/qsat.cpp elim_rec/elim; src/qe/qe.cpp extract_vars.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *qsatSearchCall) extractVariables(q ast.ASTHandle, body ast.ASTRef, vars *[]ast.ASTRef) error {
	s := c.owner
	m := s.manager
	n, err := m.GetNumDeclarations(q)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		sort, err := m.GetDeclarationSort(q, i)
		if err != nil {
			return err
		}
		v, err := c.hold(m.MkFreshConst("x", sort))
		if err != nil {
			return err
		}
		if err = s.appendRef(vars, v); err != nil {
			return err
		}
	}
	values := make([]ast.ASTHandle, len(*vars))
	for i, r := range *vars {
		values[i], err = m.RefGet(r)
		if err != nil {
			return err
		}
	}
	tmp, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	sub, err := rewriter.NewVarSubstitution(m, true)
	if sub != nil {
		c.substitutions = append(c.substitutions, sub)
	}
	if err != nil {
		return err
	}
	h, err := m.RefGet(body)
	if err != nil {
		return err
	}
	if err = sub.Substitute(h, values, tmp); err != nil {
		return err
	}
	shift, err := rewriter.NewInverseVarShifter(m)
	if shift != nil {
		c.shifters = append(c.shifters, shift)
	}
	if err != nil {
		return err
	}
	h, err = m.RefGet(tmp)
	if err != nil {
		return err
	}
	return shift.InverseShift(h, uint32(len(values)), body)
}
func (s *qsatSearch) eliminate(c *qsatSearchCall, vars []ast.ASTRef, original ast.ASTHandle) (ast.ASTHandle, error) {
	if _, err := c.ref(original); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := s.resetState(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := s.preparation.appendBlock(nil); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := s.preparation.appendBlock(vars); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := s.preparation.initializeLevels(); err != nil {
		return ast.ASTHandle{}, err
	}
	h, err := c.hold(s.manager.PushNot(original))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formula, err := c.ref(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.preparation.abstraction.emit("elim", 1030, func(w io.Writer) error {
		if e := writeExpressionVector(s.manager, s.config, w, vars); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " ")
		if e := writeExpression(s.manager, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	// Source elim initializes only the quantified variables, then its atom
	// abstraction blindly finds levels for free Boolean constants. The old
	// executable dereferences a missing entry for e.g. exists x. x or free.
	// Compute the existing dependency levels first: explicitly bound levels
	// stay intact and free parameters receive the ordinary empty sentinel.
	if _, err = s.preparation.abstraction.ComputeLevel(h); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.prepareKernels(c, formula); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.preparation.abstraction.emit("elim", 1038, func(w io.Writer) error {
		_, _ = io.WriteString(w, "ex: ")
		h, e := s.manager.RefGet(formula)
		if e != nil {
			return e
		}
		if e = writeExpression(s.manager, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	status, err := s.checkLoop()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	h, err = c.conjunction(s.answers)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.preparation.abstraction.emit("elim", 1041, func(w io.Writer) error {
		_, _ = io.WriteString(w, "ans: ")
		if e := writeExpression(s.manager, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\nFree vars: ")
		if e := writeExpressionVector(s.manager, s.config, w, s.freeVariables); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if status != lbool.False {
		return original, nil
	}
	seen := make(map[ast.ASTHandle]bool)
	for i := 0; i < len(s.freeVariables); i++ {
		v, err := s.manager.RefGet(s.freeVariables[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !seen[v] {
			seen[v] = true
			continue
		}
		last := len(s.freeVariables) - 1
		back, err := s.manager.RefGet(s.freeVariables[last])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = s.manager.SetRef(s.freeVariables[i], back); err != nil {
			return ast.ASTHandle{}, err
		}
		if err = s.manager.DelRef(&s.freeVariables[last]); err != nil {
			return ast.ASTHandle{}, err
		}
		s.freeVariables = s.freeVariables[:last]
		i--
	}
	bound := make([]ast.ASTHandle, len(s.freeVariables))
	for i, r := range s.freeVariables {
		bound[i], err = s.manager.RefGet(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	a, err := ast.NewExprAbstractor(s.manager)
	if a != nil {
		c.abstractors = append(c.abstractors, a)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	out, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = a.MkExists(bound, h, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.manager.RefGet(out)
}
func (s *qsatSearch) eliminateRecursive(root ast.ASTHandle, out ast.ASTRef) (err error) {
	c, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(c, &err)
	if _, err = c.ref(root); err != nil {
		return err
	}
	h, err := s.eliminateRecursiveFrame(c, root)
	if err != nil {
		return err
	}
	return s.manager.SetRef(out, h)
}
func (s *qsatSearch) eliminateRecursiveFrame(c *qsatSearchCall, root ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.manager
	visited := make(map[ast.ASTHandle]ast.ASTHandle)
	todo := []ast.ASTHandle{root}
	for len(todo) > 0 {
		if err := s.checkpoint(); err != nil {
			return ast.ASTHandle{}, err
		}
		h := todo[len(todo)-1]
		if _, ok := visited[h]; ok {
			todo = todo[:len(todo)-1]
			continue
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		switch kind {
		case ast.ASTApp:
			n, err := m.GetNumArgs(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args := make([]ast.ASTHandle, 0, n)
			ready := true
			for i := uint32(0); i < n; i++ {
				child, err := m.GetArg(h, i)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if v, ok := visited[child]; ok {
					args = append(args, v)
				} else {
					todo = append(todo, child)
					ready = false
				}
			}
			if ready {
				decl, err := m.GetDecl(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				v, err := c.hold(m.MkApp(decl, args))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				todo = todo[:len(todo)-1]
				visited[h] = v
			}
		case ast.ASTQuantifier:
			forall, err := m.IsForall(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			tmp, err := c.ref(body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			vars := c.vector()
			if err = c.extractVariables(h, tmp, vars); err != nil {
				return ast.ASTHandle{}, err
			}
			body, err = m.RefGet(tmp)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if err = s.preparation.abstraction.emit("elim_rec", 999, func(w io.Writer) error {
				if e := writeExpressionVector(m, s.config, w, *vars); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " ")
				if e := writeExpression(m, s.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " ")
				if e := writeExpression(m, s.config, w, body); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return ast.ASTHandle{}, err
			}
			body, err = s.eliminateRecursiveFrame(c, body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if forall {
				body, err = c.hold(m.PushNot(body))
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			body, err = s.eliminate(c, *vars, body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if forall {
				body, err = c.hold(m.PushNot(body))
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			if _, err = c.ref(body); err != nil {
				return ast.ASTHandle{}, err
			}
			visited[h] = body
			todo = todo[:len(todo)-1]
		default:
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	return visited[root], nil
}
