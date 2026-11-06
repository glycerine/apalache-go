package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// StoreExtraction owns the source implicant solver's persistent store-naming
// memo and reverse mappings. The context is borrowed. Quantifiers are opaque.
// Source: duality_rpfp.cpp:405-431,2883-2901.
type StoreExtraction struct {
	context     *Context
	memo        map[ast.ASTHandle]*Expr
	renaming    map[ast.ASTHandle]ast.ASTHandle
	constraints []*Expr
	pins        []*Expr
	closed      bool
}

func NewStoreExtraction(context *Context) (*StoreExtraction, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	return &StoreExtraction{context: context, memo: make(map[ast.ASTHandle]*Expr), renaming: make(map[ast.ASTHandle]ast.ASTHandle)}, nil
}
func (s *StoreExtraction) open() error {
	if s == nil || s.closed {
		return ErrWrapperOwner
	}
	return s.context.open()
}
func (s *StoreExtraction) keep(h ast.ASTHandle, err error) (*Expr, error) {
	if err != nil {
		return nil, err
	}
	term, err := NewExpr(s.context, h)
	if err != nil {
		return nil, err
	}
	s.pins = append(s.pins, term)
	return term, nil
}
func (s *StoreExtraction) Extract(formula *Expr) (*Expr, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != s.context {
		return nil, ErrContextMismatch
	}
	root, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	m := s.context.manager
	var visit func(ast.ASTHandle) (*Expr, error)
	visit = func(h ast.ASTHandle) (*Expr, error) {
		if found, ok := s.memo[h]; ok {
			return found, nil
		}
		// Own every input memo key, including nodes whose rewritten result no
		// longer references that input (otherwise arena slot reuse could alias it).
		input, err := s.keep(h, nil)
		if err != nil {
			return nil, err
		}
		result := input
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTApp {
			n, err := m.GetNumArgs(h)
			if err != nil {
				return nil, err
			}
			args := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				child, err := m.GetArg(h, i)
				if err != nil {
					return nil, err
				}
				output, err := visit(child)
				if err != nil {
					return nil, err
				}
				args[i], err = output.Raw()
				if err != nil {
					return nil, err
				}
			}
			decl, err := m.GetDecl(h)
			if err != nil {
				return nil, err
			}
			result, err = s.keep(m.MkApp(decl, args))
			if err != nil {
				return nil, err
			}
			family, err := m.GetFamilyID(h)
			if err != nil {
				return nil, err
			}
			op, err := m.GetDeclKind(h)
			if err != nil {
				return nil, err
			}
			array, err := m.ArrayFamilyID()
			if err != nil {
				return nil, err
			}
			if family == array && op == ast.ASTArrayOpStore {
				rebuilt, err := result.Raw()
				if err != nil {
					return nil, err
				}
				sort, err := m.GetSort(rebuilt)
				if err != nil {
					return nil, err
				}
				fresh, err := s.keep(m.MkFreshConst("@arr", sort))
				if err != nil {
					return nil, err
				}
				name, err := fresh.Raw()
				if err != nil {
					return nil, err
				}
				equality, err := s.keep(m.MkEq(name, rebuilt))
				if err != nil {
					return nil, err
				}
				s.constraints = append(s.constraints, equality)
				s.renaming[name] = rebuilt
				result = fresh
			}
		}
		s.memo[h] = result
		return result, nil
	}
	result, err := visit(root)
	if err != nil {
		return nil, err
	}
	return result.Clone()
}

// NamingConstraints returns independently owned copies in source creation
// order. The persistent cache keeps the originals across native solver pops.
func (s *StoreExtraction) NamingConstraints() (out []*Expr, result error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	var owned []*Expr
	defer func() {
		if result != nil {
			for _, term := range owned {
				result = errors.Join(result, term.Close())
			}
			out = nil
		}
	}()
	for _, constraint := range s.constraints {
		term, err := constraint.Clone()
		if err != nil {
			return nil, err
		}
		owned = append(owned, term)
	}
	return owned, nil
}

// Undo performs source SubstRec once: a replacement RHS is not visited again.
// Nested store names can therefore require another call to restore fully.
func (s *StoreExtraction) Undo(formula *Expr) (out *Expr, result error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != s.context {
		return nil, ErrContextMismatch
	}
	root, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle, len(s.renaming))
	for name, value := range s.renaming {
		memo[name] = value
	}
	ref, err := substituteExpression(s.context, root, memo)
	if err != nil {
		return nil, err
	}
	m := s.context.manager
	defer func() {
		result = errors.Join(result, m.DelRef(&ref))
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	h, err := m.RefGet(ref)
	if err != nil {
		return nil, err
	}
	return NewExpr(s.context, h)
}
func (s *StoreExtraction) Close() error {
	if s == nil || s.closed {
		return nil
	}
	for len(s.pins) > 0 {
		n := len(s.pins) - 1
		if err := s.pins[n].Close(); err != nil {
			return err
		}
		s.pins = s.pins[:n]
	}
	s.memo = nil
	s.renaming = nil
	s.constraints = nil
	s.closed = true
	return nil
}
