package smtcore

// Source: src/smt/expr_context_simplifier.cpp, simplify_model_based.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type strongContextFrame struct {
	expression, name ast.ASTHandle
	parent, self     uint64
	checked          bool
}
type strongContextResult struct {
	parent     uint64
	expression ast.ASTHandle
}

// Simplify returns an owned reference to the input simplified under the
// currently asserted context. The caller must release the returned reference.
func (s *ExprStrongContextSimplifier) Simplify(formula ast.ASTHandle) (result ast.ASTRef, err error) {
	if err = s.open(); err != nil {
		return result, err
	}
	m := s.manager
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	sort, err := m.GetSort(formula)
	if err != nil {
		return result, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	quantified, err := m.HasQuantifiers(formula)
	if err != nil {
		return result, err
	}
	if sort != boolean || quantified {
		return m.NewRef(formula)
	}
	assignments, status, err := s.captureAssignments(formula)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, assignments.Close()) }()
	if status == lbool.False {
		falsity, e := m.MkFalse()
		if e != nil {
			return result, e
		}
		return m.NewRef(falsity)
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
	}()
	keep := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		ref, e := m.NewRef(h)
		if e != nil {
			return h, e
		}
		refs = append(refs, ref)
		return h, nil
	}
	newName := func(id uint64) (ast.ASTHandle, error) {
		n, e := m.ArithmeticIntegerNumeralText(strconv.FormatUint(id, 10))
		if e != nil {
			return n, e
		}
		n, e = keep(n)
		if e != nil {
			return n, e
		}
		decl, e := m.RefGet(s.name)
		if e != nil {
			return n, e
		}
		n, e = m.MkApp(decl, []ast.ASTHandle{n})
		if e != nil {
			return n, e
		}
		return keep(n)
	}
	isBool := func(h ast.ASTHandle) (bool, error) { t, e := m.GetSort(h); return t == boolean, e }
	depth := 0
	defer func() {
		for depth > 0 {
			e := s.solver.Pop(1)
			err = errors.Join(err, e)
			if e != nil {
				break
			}
			depth--
		}
	}()
	push := func() error {
		if e := s.solver.Push(); e != nil {
			return e
		}
		depth++
		return nil
	}
	pop := func() error {
		if e := s.solver.Pop(1); e != nil {
			return e
		}
		depth--
		return nil
	}
	if err = push(); err != nil {
		return result, err
	}
	id := uint64(1)
	rootName, err := newName(id)
	if err != nil {
		return result, err
	}
	id++
	equality, err := m.MkIff(formula, rootName)
	if err != nil {
		return result, err
	}
	equality, err = keep(equality)
	if err != nil {
		return result, err
	}
	difference, err := m.MkNot(equality)
	if err != nil {
		return result, err
	}
	difference, err = keep(difference)
	if err != nil {
		return result, err
	}
	if err = s.solver.AssertFormula(difference); err != nil {
		return result, err
	}
	if err = push(); err != nil {
		return result, err
	}
	frames := []strongContextFrame{{expression: formula, name: rootName}}
	cache := make(map[ast.ASTHandle]strongContextResult)
	var path uint64
	for len(frames) != 0 {
		index := len(frames) - 1
		f := &frames[index]
		expression, name, parent := f.expression, f.name, f.parent
		var replacement ast.ASTHandle
		done := false
		if _, ok := cache[expression]; ok {
			done = true
		}
		if !done {
			b, e := isBool(expression)
			if e != nil {
				return result, e
			}
			if !b {
				replacement = expression
				done = true
			}
		}
		if !done && !f.checked {
			value, ok := assignments.values[expression]
			if !ok {
				value = lbool.Undef
			}
			if value != lbool.False {
				truth, e := m.MkTrue()
				if e != nil {
					return result, e
				}
				forced, e := s.isForced(name, truth)
				if e != nil {
					return result, e
				}
				if forced {
					replacement = truth
					done = true
				}
			}
			if !done && value != lbool.True {
				falsity, e := m.MkFalse()
				if e != nil {
					return result, e
				}
				forced, e := s.isForced(name, falsity)
				if e != nil {
					return result, e
				}
				if forced {
					replacement = falsity
					done = true
				}
			}
		}
		if !done {
			kind, e := m.GetKind(expression)
			if e != nil {
				return result, e
			}
			if kind != ast.ASTApp {
				replacement = expression
				done = true
			}
		}
		if !done {
			if !f.checked {
				path++
				f.self = path
				f.checked = true
			}
			self := f.self
			count, e := m.GetNumArgs(expression)
			if e != nil {
				return result, e
			}
			args := make([]ast.ASTHandle, count)
			var child, childName ast.ASTHandle
			for i := range args {
				arg, e := m.GetArg(expression, uint32(i))
				if e != nil {
					return result, e
				}
				args[i] = arg
				if cached, ok := cache[arg]; ok {
					if cached.parent == self {
						args[i] = cached.expression
					}
					continue
				}
				b, e := isBool(arg)
				if e != nil {
					return result, e
				}
				if b && childName == (ast.ASTHandle{}) {
					child, childName = arg, ast.ASTHandle{}
					childName, e = newName(id)
					if e != nil {
						return result, e
					}
					id++
					args[i] = childName
				}
			}
			decl, e := m.GetDecl(expression)
			if e != nil {
				return result, e
			}
			replacement, e = m.MkApp(decl, args)
			if e != nil {
				return result, e
			}
			replacement, e = keep(replacement)
			if e != nil {
				return result, e
			}
			if childName != (ast.ASTHandle{}) {
				frames = append(frames, strongContextFrame{expression: child, name: childName, parent: self})
				if err = push(); err != nil {
					return result, err
				}
				eq, e := m.MkEq(replacement, name)
				if e != nil {
					return result, e
				}
				eq, e = keep(eq)
				if e != nil {
					return result, e
				}
				if err = s.solver.AssertFormula(eq); err != nil {
					return result, err
				}
				continue
			}
		}
		if replacement != (ast.ASTHandle{}) {
			replacement, err = keep(replacement)
			if err != nil {
				return result, err
			}
			cache[expression] = strongContextResult{parent: parent, expression: replacement}
		}
		frames = frames[:index]
		if err = pop(); err != nil {
			return result, err
		}
	}
	root, ok := cache[formula]
	if !ok {
		return result, ast.ErrASTHandle
	}
	if err = pop(); err != nil {
		return result, err
	}
	return m.NewRef(root.expression)
}
