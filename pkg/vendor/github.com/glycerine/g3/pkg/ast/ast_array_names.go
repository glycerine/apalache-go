package ast

// Source: src/ast/array_decl_plugin.cpp:518-541,566-587.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

// GetArraySortNames appends the source array sort name for every logic.
func (m *ASTManager) GetArraySortNames(names *ASTBuiltinNames, _ usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.arrayPlugin == nil {
		return ErrASTHandle
	}
	return m.appendBuiltinName(names, "Array", 0)
}

// GetArrayOpNames appends the two SMT-LIB array operations for every logic and
// the source extensions only for the null, HORN, and ALL logic selections.
func (m *ASTManager) GetArrayOpNames(names *ASTBuiltinNames, logic usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.arrayPlugin == nil {
		return ErrASTHandle
	}
	for _, entry := range [...]struct {
		text string
		kind int32
	}{{"store", ASTArrayOpStore}, {"select", ASTArrayOpSelect}} {
		if e := m.appendBuiltinName(names, entry.text, entry.kind); e != nil {
			return e
		}
	}
	extended := logic.IsNull()
	if !extended {
		for _, text := range [...]string{"HORN", "ALL"} {
			yes, e := m.Symbols.EqualString(logic, &text)
			if e != nil {
				return e
			}
			if yes {
				extended = true
				break
			}
		}
	}
	if !extended {
		return nil
	}
	for _, entry := range [...]struct {
		text string
		kind int32
	}{
		{"const", ASTArrayOpConst}, {"map", ASTArrayOpMap}, {"default", ASTArrayOpDefault},
		{"union", ASTArrayOpSetUnion}, {"intersect", ASTArrayOpSetIntersect},
		{"difference", ASTArrayOpSetDifference}, {"complement", ASTArrayOpSetComplement},
		{"subset", ASTArrayOpSetSubset}, {"as-array", ASTArrayOpAsArray}, {"array-ext", ASTArrayOpExt},
	} {
		if e := m.appendBuiltinName(names, entry.text, entry.kind); e != nil {
			return e
		}
	}
	return nil
}

// IsAsArrayTree recognizes a tree whose leaves are as-array applications and
// whose internal nodes are Boolean-family ITE applications. It follows only
// the then and else children, matching array_util::is_as_array_tree.
func (m *ASTManager) IsAsArrayTree(root ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if m.arrayPlugin == nil {
		return false, ErrASTHandle
	}
	r, e := m.requireExpr(root)
	if e != nil {
		return false, e
	}
	todo := []astNodeRef{r}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if m.nodeAt(current.slot).kind != ASTApp {
			return false, nil
		}
		app := m.appAt(current.slot)
		family, kind, e := m.declIdentity(app.decl)
		if e != nil {
			return false, e
		}
		if family == m.arrayPlugin.family && kind == ASTArrayOpAsArray {
			continue
		}
		if family != 0 || kind != ASTOpIte || app.count != 3 {
			return false, nil
		}
		otherwise, e := m.readInternal(app.args, 2)
		if e != nil {
			return false, e
		}
		then, e := m.readInternal(app.args, 1)
		if e != nil {
			return false, e
		}
		todo = append(todo, otherwise, then)
	}
	return true, nil
}
