package interp

// Source checkpoint: src/interp/iz3mgr.cpp substitution and variable queries.

import "github.com/glycerine/g3/pkg/ast"

func (m *Manager) Occurs(needle, root ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	memo := make(map[ast.ASTHandle]bool)
	known := make(map[ast.ASTHandle]bool)
	var visit func(ast.ASTHandle) (bool, error)
	visit = func(node ast.ASTHandle) (bool, error) {
		if known[node] {
			return memo[node], nil
		}
		known[node] = true
		if node == needle {
			memo[node] = true
			return true, nil
		}
		count, err := m.NumArgs(node)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count; i++ {
			child, e := m.Arg(node, i)
			if e != nil {
				return false, e
			}
			found, e := visit(child)
			if e != nil {
				return false, e
			}
			if found {
				memo[node] = true
				return true, nil
			}
		}
		return false, nil
	}
	return visit(root)
}

func (m *Manager) substitute(root ast.ASTHandle, direct map[ast.ASTHandle]ast.ASTHandle, memo map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	if replacement, found := direct[root]; found {
		return replacement, nil
	}
	if result, found := memo[root]; found {
		return result, nil
	}
	count, err := m.NumArgs(root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		child, e := m.Arg(root, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args[i], e = m.substitute(child, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	op, err := m.Operator(root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	if op == OperatorEqual && len(args) == 2 && args[0] == args[1] {
		result, err = m.Make(OperatorTrue)
	} else {
		result, err = m.Clone(root, args)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	memo[root] = result
	return result, nil
}

func (m *Manager) SubstituteOne(variable, replacement, root ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.substitute(root, map[ast.ASTHandle]ast.ASTHandle{variable: replacement}, make(map[ast.ASTHandle]ast.ASTHandle))
}

func (m *Manager) Substitute(replacements map[ast.ASTHandle]ast.ASTHandle, root ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if replacements == nil {
		replacements = map[ast.ASTHandle]ast.ASTHandle{}
	}
	return m.substitute(root, replacements, make(map[ast.ASTHandle]ast.ASTHandle))
}

func (m *Manager) NumFreeVariables(root ast.ASTHandle) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	used, err := ast.NewASTUsedVars(m.ast)
	if err != nil {
		return 0, err
	}
	if err = used.Collect(root); err != nil {
		return 0, err
	}
	return used.NumVars(), nil
}
