package smtcore

// Source: src/smt/smt_model_finder.cpp:2867-2908.

import "github.com/glycerine/g3/pkg/ast"

// selectDefinition is the m_fs.insert operation after greedy pushes its two
// quantifier-set scopes. Definitions borrow both AST handles.
func (s *hintSearchState) selectDefinition(f, def ast.ASTHandle) error {
	if s == nil || s.definitions == nil {
		return ast.ErrASTDomain
	}
	if _, present := s.definitions[f]; present {
		return ast.ErrASTDomain
	}
	s.definitions[f] = def
	return nil
}

func (s *hintSearchState) forgetDefinition(f ast.ASTHandle) {
	if s != nil {
		delete(s.definitions, f)
	}
}

// isCyclic follows hint_solver::is_cyclic's fixed-point growth of m_acyclic.
// The source treats a function as acyclic only when no selected, still
// unclassified function occurs anywhere in its definition expression.
func (s *hintSearchState) isCyclic(m *ast.ASTManager) (bool, error) {
	if s == nil || m == nil || s.definitions == nil {
		return false, ast.ErrASTDomain
	}
	acyclic := make(map[ast.ASTHandle]struct{}, len(s.definitions))
	for {
		before := len(acyclic)
		if before == len(s.definitions) {
			return false, nil
		}
		for f, def := range s.definitions {
			if _, known := acyclic[f]; known {
				continue
			}
			ready, e := s.definitionAcyclic(m, def, acyclic)
			if e != nil {
				return false, e
			}
			if ready {
				acyclic[f] = struct{}{}
			}
		}
		if len(acyclic) == before {
			return true, nil
		}
	}
}

// definitionAcyclic is the concrete occurs_check traversal. The visited map
// replaces expr_mark; marking every reached AST identity gives the same
// occurrence result as the source's reference-count-aware mark.
func (s *hintSearchState) definitionAcyclic(m *ast.ASTManager, def ast.ASTHandle, acyclic map[ast.ASTHandle]struct{}) (bool, error) {
	seen := make(map[ast.ASTHandle]struct{})
	todo := []ast.ASTHandle{def}
	for len(todo) != 0 {
		i := len(todo) - 1
		n := todo[i]
		todo = todo[:i]
		if _, done := seen[n]; done {
			continue
		}
		seen[n] = struct{}{}
		kind, e := m.GetKind(n)
		if e != nil {
			return false, e
		}
		switch kind {
		case ast.ASTVar:
		case ast.ASTApp:
			decl, e := m.GetDecl(n)
			if e != nil {
				return false, e
			}
			if _, selected := s.definitions[decl]; selected {
				if _, ready := acyclic[decl]; !ready {
					return false, nil
				}
			}
			count, e := m.GetNumArgs(n)
			if e != nil {
				return false, e
			}
			for j := count; j > 0; j-- {
				child, e := m.GetArg(n, j-1)
				if e != nil {
					return false, e
				}
				todo = append(todo, child)
			}
		case ast.ASTQuantifier:
			// for_each_expr visits body, patterns and no-patterns in order.
			noPatterns, e := m.GetNumNoPatterns(n)
			if e != nil {
				return false, e
			}
			for j := noPatterns; j > 0; j-- {
				child, e := m.GetNoPattern(n, j-1)
				if e != nil {
					return false, e
				}
				todo = append(todo, child)
			}
			patterns, e := m.GetNumPatterns(n)
			if e != nil {
				return false, e
			}
			for j := patterns; j > 0; j-- {
				child, e := m.GetPattern(n, j-1)
				if e != nil {
					return false, e
				}
				todo = append(todo, child)
			}
			body, e := m.GetQuantifierBody(n)
			if e != nil {
				return false, e
			}
			todo = append(todo, body)
		default:
			return false, ast.ErrASTDomain
		}
	}
	return true, nil
}
