package substitution

// Source: src/ast/substitution/substitution_tree.cpp:619-861;
// retained STV_UNIF specialization.

import "github.com/glycerine/g3/pkg/ast"

// TreeVisitor receives each indexed expression that unifies with the query.
// Its substitution is valid during Visit and is restored while backtracking.
type TreeVisitor interface {
	TreeSubstitution() *Substitution
	VisitTreeExpression(ast.ASTHandle) (bool, error)
}

type treeVisit struct {
	tree                                 *SubstitutionTree
	subst                                *Substitution
	visitor                              TreeVisitor
	inOffset, treeOffset, registerOffset uint32
	stack                                []*treeNode
	todo                                 []unifyPair
}

func (v *treeVisit) find(p ExprOffset) (ExprOffset, error) {
	for {
		kind, e := v.tree.manager.GetKind(p.expr)
		if e != nil {
			return ExprOffset{}, e
		}
		if kind != ast.ASTVar {
			return p, nil
		}
		i, e := v.tree.manager.GetVarIndex(p.expr)
		if e != nil {
			return ExprOffset{}, e
		}
		next, ok, e := v.subst.Find(i, p.offset)
		if e != nil {
			return ExprOffset{}, e
		}
		if !ok {
			return p, nil
		}
		p = next
	}
}
func (v *treeVisit) bindVariable(variable ast.ASTHandle, offset uint32, p ExprOffset) error {
	i, e := v.tree.manager.GetVarIndex(variable)
	if e != nil {
		return e
	}
	return v.subst.Insert(i, offset, p)
}
func (v *treeVisit) unify(left, right ExprOffset) (bool, error) {
	v.todo = v.todo[:0]
	v.todo = append(v.todo, unifyPair{left, right})
	for len(v.todo) != 0 {
		i := len(v.todo) - 1
		p := v.todo[i]
		v.todo = v.todo[:i]
		var e error
		left, e = v.find(p.left)
		if e != nil {
			return false, e
		}
		right, e = v.find(p.right)
		if e != nil {
			return false, e
		}
		if left == right {
			continue
		}
		lk, e := v.tree.manager.GetKind(left.expr)
		if e != nil {
			return false, e
		}
		rk, e := v.tree.manager.GetKind(right.expr)
		if e != nil {
			return false, e
		}
		if lk == ast.ASTQuantifier || rk == ast.ASTQuantifier {
			return false, ast.ErrASTDomain
		}
		lv, rv := lk == ast.ASTVar, rk == ast.ASTVar
		if lv && rv {
			if right.offset == v.registerOffset {
				left, right = right, left
			}
			if e = v.bindVariable(left.expr, left.offset, right); e != nil {
				return false, e
			}
		} else if lv {
			if e = v.bindVariable(left.expr, left.offset, right); e != nil {
				return false, e
			}
		} else if rv {
			if e = v.bindVariable(right.expr, right.offset, left); e != nil {
				return false, e
			}
		} else {
			if lk != ast.ASTApp || rk != ast.ASTApp {
				return false, ast.ErrASTDomain
			}
			ld, e := v.tree.manager.GetDecl(left.expr)
			if e != nil {
				return false, e
			}
			rd, e := v.tree.manager.GetDecl(right.expr)
			if e != nil {
				return false, e
			}
			ln, e := v.tree.manager.GetNumArgs(left.expr)
			if e != nil {
				return false, e
			}
			rn, e := v.tree.manager.GetNumArgs(right.expr)
			if e != nil {
				return false, e
			}
			if ld != rd || ln != rn {
				return false, nil
			}
			for j := ln; j > 0; j-- {
				la, e := v.tree.manager.GetArg(left.expr, j-1)
				if e != nil {
					return false, e
				}
				ra, e := v.tree.manager.GetArg(right.expr, j-1)
				if e != nil {
					return false, e
				}
				v.todo = append(v.todo, unifyPair{NewExprOffset(la, left.offset), NewExprOffset(ra, right.offset)})
			}
		}
	}
	return true, nil
}
func (v *treeVisit) visitInstructions(sv []treeSubst) (bool, error) {
	for _, s := range sv {
		kind, e := v.tree.manager.GetKind(s.value)
		if e != nil {
			return false, e
		}
		offset := v.registerOffset
		if kind == ast.ASTVar {
			offset = v.treeOffset
		}
		if ok, e := v.unify(NewExprOffset(s.register, v.registerOffset), NewExprOffset(s.value, offset)); e != nil || !ok {
			return ok, e
		}
	}
	return true, nil
}
func (v *treeVisit) popScope() error { return v.subst.PopScope(1) }
func (v *treeVisit) backtrack() (bool, error) {
	for len(v.stack) != 0 {
		if e := v.popScope(); e != nil {
			return false, e
		}
		i := len(v.stack) - 1
		n := v.stack[i]
		if n.next != nil {
			v.stack[i] = n.next
			return true, nil
		}
		v.stack = v.stack[:i]
	}
	return false, nil
}
func (v *treeVisit) clearStack() error {
	for len(v.stack) != 0 {
		if e := v.popScope(); e != nil {
			return e
		}
		v.stack = v.stack[:len(v.stack)-1]
	}
	return v.popScope()
}
func (v *treeVisit) visitRoot(query ast.ASTHandle, root *treeNode) (result error) {
	v.stack = v.stack[:0]
	v.stack = append(v.stack, root)
	if e := v.subst.PushScope(); e != nil {
		return e
	}
	if e := v.subst.Insert(0, v.registerOffset, NewExprOffset(query, v.inOffset)); e != nil {
		_ = v.popScope()
		return e
	}
	defer func() {
		if len(v.stack) != 0 {
			if e := v.clearStack(); result == nil {
				result = e
			}
		}
	}()
	for {
		n := v.stack[len(v.stack)-1]
		if e := v.subst.PushScope(); e != nil {
			return e
		}
		ok, e := v.visitInstructions(n.subst)
		if e != nil {
			return e
		}
		if ok {
			if n.leaf {
				acyclic, e := v.subst.Acyclic()
				if e != nil {
					return e
				}
				if acyclic {
					more, e := v.visitor.VisitTreeExpression(n.expr)
					if e != nil {
						return e
					}
					if !more {
						return v.clearStack()
					}
				}
				more, e := v.backtrack()
				if e != nil {
					return e
				}
				if !more {
					break
				}
			} else {
				v.stack = append(v.stack, n.child)
			}
		} else {
			more, e := v.backtrack()
			if e != nil {
				return e
			}
			if !more {
				break
			}
		}
	}
	return v.clearStack()
}
func (v *treeVisit) visitVariables(query ast.ASTHandle) (bool, error) {
	sort, e := v.tree.manager.GetSort(query)
	if e != nil {
		return false, e
	}
	for _, candidate := range v.tree.variables[sort] {
		if e = v.subst.PushScope(); e != nil {
			return false, e
		}
		ok, e := v.unify(NewExprOffset(candidate, v.treeOffset), NewExprOffset(query, v.inOffset))
		if e != nil {
			_ = v.popScope()
			return false, e
		}
		if ok {
			acyclic, e := v.subst.Acyclic()
			if e != nil {
				_ = v.popScope()
				return false, e
			}
			if acyclic {
				more, e := v.visitor.VisitTreeExpression(candidate)
				if e != nil {
					_ = v.popScope()
					return false, e
				}
				if !more {
					if e = v.popScope(); e != nil {
						return false, e
					}
					return false, nil
				}
			}
		}
		if e = v.popScope(); e != nil {
			return false, e
		}
	}
	return true, nil
}

// Unify visits every indexed expression that unifies with query using source
// offsets 0 (query), 1 (tree), and 2 (compiled registers).
func (t *SubstitutionTree) Unify(query ast.ASTHandle, visitor TreeVisitor) error {
	return t.UnifyOffsets(query, visitor, 0, 1, 2)
}
func (t *SubstitutionTree) UnifyOffsets(query ast.ASTHandle, visitor TreeVisitor, inOffset, treeOffset, registerOffset uint32) error {
	if e := t.open(); e != nil {
		return e
	}
	if visitor == nil {
		return ast.ErrASTHandle
	}
	s := visitor.TreeSubstitution()
	if s == nil {
		return ast.ErrASTHandle
	}
	if e := s.open(); e != nil {
		return e
	}
	if s.manager != t.manager {
		return ast.ErrASTHandle
	}
	need := registerOffset + 1
	if need == 0 {
		return ast.ErrASTDomain
	}
	if s.bindings.offsets < need {
		return ast.ErrASTDomain
	}
	if e := s.ReserveVariables(t.maxReg + 1); e != nil {
		return e
	}
	v := treeVisit{tree: t, subst: s, visitor: visitor, inOffset: inOffset, treeOffset: treeOffset, registerOffset: registerOffset}
	more, e := v.visitVariables(query)
	if e != nil || !more {
		return e
	}
	kind, e := t.manager.GetKind(query)
	if e != nil {
		return e
	}
	if kind == ast.ASTApp {
		decl, e := t.manager.GetDecl(query)
		if e != nil {
			return e
		}
		if root := t.roots[decl]; root != nil {
			return v.visitRoot(query, root)
		}
		return nil
	}
	if kind != ast.ASTVar {
		return ast.ErrASTDomain
	}
	for _, root := range t.roots {
		if root == nil || len(root.subst) == 0 {
			continue
		}
		rs, e := t.manager.GetSort(root.subst[0].register)
		if e != nil {
			return e
		}
		qs, e := t.manager.GetSort(query)
		if e != nil {
			return e
		}
		if rs == qs {
			if e = v.visitRoot(query, root); e != nil {
				return e
			}
		}
	}
	return nil
}
