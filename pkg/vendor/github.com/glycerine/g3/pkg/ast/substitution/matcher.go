package substitution

// Source: src/ast/substitution/matcher.cpp;
// src/ast/substitution/matcher.h.

import "github.com/glycerine/g3/pkg/ast"

type matchPair struct{ pattern, instance ast.ASTHandle }

// Matcher checks whether instance is an instance of pattern and appends the
// source offset-0 to offset-1 bindings to the supplied substitution.
type Matcher struct{ todo []matchPair }

func (m *Matcher) Match(pattern, instance ast.ASTHandle, s *Substitution) (bool, error) {
	if s == nil {
		return false, ast.ErrASTHandle
	}
	if err := s.open(); err != nil {
		return false, err
	}
	m.todo = m.todo[:0]
	m.todo = append(m.todo, matchPair{pattern, instance})
	for len(m.todo) != 0 {
		i := len(m.todo) - 1
		p := m.todo[i]
		pk, e := s.manager.GetKind(p.pattern)
		if e != nil {
			return false, e
		}
		if pk == ast.ASTVar {
			index, e := s.manager.GetVarIndex(p.pattern)
			if e != nil {
				return false, e
			}
			r, found, e := s.Find(index, 0)
			if e != nil {
				return false, e
			}
			if found {
				if r.expr != p.instance {
					return false, nil
				}
			} else if e = s.Insert(index, 0, NewExprOffset(p.instance, 1)); e != nil {
				return false, e
			}
			m.todo = m.todo[:i]
			continue
		}
		ik, e := s.manager.GetKind(p.instance)
		if e != nil {
			return false, e
		}
		if ik == ast.ASTVar || pk != ast.ASTApp || ik != ast.ASTApp {
			return false, nil
		}
		pd, e := s.manager.GetDecl(p.pattern)
		if e != nil {
			return false, e
		}
		id, e := s.manager.GetDecl(p.instance)
		if e != nil {
			return false, e
		}
		if pd != id {
			return false, nil
		}
		pn, e := s.manager.GetNumArgs(p.pattern)
		if e != nil {
			return false, e
		}
		in, e := s.manager.GetNumArgs(p.instance)
		if e != nil {
			return false, e
		}
		if pn != in {
			return false, nil
		}
		m.todo = m.todo[:i]
		for j := pn; j > 0; j-- {
			pa, e := s.manager.GetArg(p.pattern, j-1)
			if e != nil {
				return false, e
			}
			ia, e := s.manager.GetArg(p.instance, j-1)
			if e != nil {
				return false, e
			}
			m.todo = append(m.todo, matchPair{pa, ia})
		}
	}
	return true, nil
}
