package tactic

// Source: src/sat/tactic/atom2bool_var.cpp:45-126;
// src/ast/for_each_expr.h:26-103,134-136.

import "github.com/glycerine/g3/pkg/ast"

func (tr *GoalTranslator) collectInterface(formulas []ast.ASTHandle) (err error) {
	return tr.collectInterfaceWithDependencies(formulas, nil)
}
func (tr *GoalTranslator) collectInterfaceWithDependencies(formulas, dependencies []ast.ASTHandle) (err error) {
	fvisited := make(map[ast.ASTHandle]bool)
	tvisited := make(map[ast.ASTHandle]bool)
	for _, dependency := range dependencies {
		if err = tr.collectGeneralInterface(dependency, tvisited); err != nil {
			return err
		}
	}
	// The outer todo remains separate because a general expression walk may
	// start while other formulas are still pending.
	todo := GoalTranslator{solver: tr.solver}
	for _, formula := range formulas {
		if fvisited[formula] {
			continue
		}
		fvisited[formula] = true
		if e := todo.pushFrame(satGoalFrame{term: formula}); e != nil {
			return e
		}
		for len(todo.frames) != 0 {
			i := len(todo.frames) - 1
			h := todo.frames[i].term
			todo.frames = todo.frames[:i]
			uninterpreted, e := tr.isUninterpretedConstant(h)
			if e != nil {
				return e
			}
			if uninterpreted {
				continue
			}
			kind, e := tr.manager.GetKind(h)
			if e != nil {
				return e
			}
			basic := false
			var count uint32
			if kind == ast.ASTApp {
				family, e := tr.manager.GetFamilyID(h)
				if e != nil {
					return e
				}
				count, e = tr.manager.GetNumArgs(h)
				if e != nil {
					return e
				}
				basic = family == 0 && count > 0
			}
			if basic {
				op, e := tr.manager.GetDeclKind(h)
				if e != nil {
					return e
				}
				descend := op == ast.ASTOpOr || op == ast.ASTOpNot || op == ast.ASTOpIff
				if op == ast.ASTOpEq || op == ast.ASTOpIte {
					arg, e := tr.manager.GetArg(h, 1)
					if e != nil {
						return e
					}
					descend, e = tr.isBool(arg)
					if e != nil {
						return e
					}
				}
				// Source deliberately omits AND from this branch; preserve it.
				if descend {
					for j := uint32(0); j < count; j++ {
						arg, e := tr.manager.GetArg(h, j)
						if e != nil {
							return e
						}
						if !fvisited[arg] {
							fvisited[arg] = true
							if e = todo.pushFrame(satGoalFrame{term: arg}); e != nil {
								return e
							}
						}
					}
				}
			} else {
				if e = tr.collectGeneralInterface(h, tvisited); e != nil {
					return e
				}
			}
		}
	}
	return nil
}

func (tr *GoalTranslator) markGeneral(h ast.ASTHandle, visited map[ast.ASTHandle]bool) (bool, error) {
	rc, e := tr.manager.GetRefCount(h)
	if e != nil {
		return false, e
	}
	if rc <= 1 {
		return false, nil
	}
	if visited[h] {
		return true, nil
	}
	visited[h] = true
	return false, nil
}
func (tr *GoalTranslator) generalChildren(h ast.ASTHandle, kind ast.ASTKind) (uint32, error) {
	switch kind {
	case ast.ASTApp:
		return tr.manager.GetNumArgs(h)
	case ast.ASTVar:
		return 0, nil
	case ast.ASTQuantifier:
		n, e := tr.manager.GetNumPatterns(h)
		if e != nil {
			return 0, e
		}
		nn, e := tr.manager.GetNumNoPatterns(h)
		return 1 + n + nn, e
	}
	return 0, ast.ErrASTDomain
}
func (tr *GoalTranslator) generalChild(h ast.ASTHandle, kind ast.ASTKind, index uint32) (ast.ASTHandle, error) {
	if kind == ast.ASTApp {
		return tr.manager.GetArg(h, index)
	}
	if kind != ast.ASTQuantifier {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if index == 0 {
		return tr.manager.GetQuantifierBody(h)
	}
	n, e := tr.manager.GetNumPatterns(h)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if index <= n {
		return tr.manager.GetPattern(h, index-1)
	}
	return tr.manager.GetNoPattern(h, index-n-1)
}
func (tr *GoalTranslator) collectGeneralInterface(h ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
	marked, e := tr.markGeneral(h, visited)
	if e != nil || marked {
		return e
	}
	if e = tr.pushFrame(satGoalFrame{term: h}); e != nil {
		return e
	}
	for len(tr.frames) != 0 {
		i := len(tr.frames) - 1
		fr := tr.frames[i]
		kind, e := tr.manager.GetKind(fr.term)
		if e != nil {
			return e
		}
		count, e := tr.generalChildren(fr.term, kind)
		if e != nil {
			return e
		}
		if fr.index < count {
			child, e := tr.generalChild(fr.term, kind, fr.index)
			if e != nil {
				return e
			}
			tr.frames[i].index++
			marked, e := tr.markGeneral(child, visited)
			if e != nil {
				return e
			}
			if marked {
				continue
			}
			if e = tr.pushFrame(satGoalFrame{term: child}); e != nil {
				return e
			}
			continue
		}
		uninterpreted, e := tr.isUninterpretedConstant(fr.term)
		if e != nil {
			return e
		}
		if uninterpreted {
			tr.interfaces[fr.term] = true
		}
		tr.frames = tr.frames[:i]
	}
	return nil
}
