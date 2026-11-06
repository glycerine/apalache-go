package qe

// Source: src/qe/qe_lite.cpp eq::occurs_var (lines 41-63).

import "github.com/glycerine/g3/pkg/ast"

// OccursVariable checks an index through application children and quantifier
// bodies. Quantifiers shift the requested index by their declaration count;
// source does not inspect pattern or no-pattern annotations here.
func OccursVariable(m *ast.ASTManager, index uint32, expression ast.ASTHandle) (bool, error) {
	if m == nil {
		return false, ast.ErrASTHandle
	}
	todo := []ast.ASTHandle{expression}
	visited := make(map[ast.ASTHandle]struct{})
	for len(todo) != 0 {
		last := len(todo) - 1
		h := todo[last]
		todo = todo[:last]
		if _, found := visited[h]; found {
			continue
		}
		visited[h] = struct{}{}
		kind, err := m.GetKind(h)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTVar:
			actual, err := m.GetVarIndex(h)
			if err != nil {
				return false, err
			}
			if actual == index {
				return true, nil
			}
		case ast.ASTApp:
			n, err := m.GetNumArgs(h)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(h, i)
				if err != nil {
					return false, err
				}
				todo = append(todo, arg)
			}
		case ast.ASTQuantifier:
			n, err := m.GetNumDeclarations(h)
			if err != nil {
				return false, err
			}
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return false, err
			}
			yes, err := OccursVariable(m, index+n, body)
			if err != nil || yes {
				return yes, err
			}
		}
	}
	return false, nil
}
