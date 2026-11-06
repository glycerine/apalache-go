package qe

// Source: src/qe/qe_datatype_plugin.cpp datatype_atoms::solve_eq.

import "github.com/glycerine/g3/pkg/ast"

func (a *datatypeEqualityAtoms) hold(m *ast.ASTManager, h ast.ASTHandle) error {
	ref, err := m.NewRef(h)
	if err == nil {
		a.refs = append(a.refs, ref)
	}
	return err
}

// solveConstructorEquality follows the first constructor path containing the
// target, carrying source recognizer conditions and accessor terms.
func (p *DatatypeSolverPlugin) solveConstructorEquality(variable *ast.ContainsApp, left, right, condition ast.ASTHandle, entry *datatypeEqualityAtoms) (value, guard ast.ASTHandle, matched bool, err error) {
	target, err := variable.Target()
	if err != nil {
		return value, guard, false, err
	}
	if left == target {
		return right, condition, true, nil
	}
	constructor, matched, err := p.manager.DatatypeConstructorApp(left)
	if err != nil || !matched {
		return value, guard, false, err
	}
	recognizer, err := p.manager.DatatypeConstructorRecognizer(constructor)
	if err != nil {
		return value, guard, false, err
	}
	accessors, err := p.manager.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return value, guard, false, err
	}
	test, err := p.manager.MkApp(recognizer, []ast.ASTHandle{right})
	if err != nil {
		return value, guard, false, err
	}
	if err = entry.hold(p.manager, test); err != nil {
		return value, guard, false, err
	}
	nextGuard, err := p.manager.MkAnd(test, condition)
	if err != nil {
		return value, guard, false, err
	}
	if err = entry.hold(p.manager, nextGuard); err != nil {
		return value, guard, false, err
	}
	for i, accessor := range accessors {
		argument, e := p.manager.GetArg(left, uint32(i))
		if e != nil {
			return value, guard, false, e
		}
		contains, e := variable.Check(argument)
		if e != nil {
			return value, guard, false, e
		}
		if !contains {
			continue
		}
		selected, e := p.manager.MkApp(accessor, []ast.ASTHandle{right})
		if e != nil {
			return value, guard, false, e
		}
		if e = entry.hold(p.manager, selected); e != nil {
			return value, guard, false, e
		}
		value, guard, matched, e = p.solveConstructorEquality(variable, argument, selected, nextGuard, entry)
		if e != nil || matched {
			return value, guard, matched, e
		}
	}
	return value, guard, false, nil
}
