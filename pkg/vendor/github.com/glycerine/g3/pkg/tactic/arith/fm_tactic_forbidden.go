package arith

// Source: src/tactic/arith/fm_tactic.cpp:474-510,795-825.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (p *FMInputParser) isOCC(formula ast.ASTHandle) (bool, error) {
	args := []ast.ASTHandle{formula}
	clause := false
	if p.AllowClauses {
		kind, err := p.manager.GetKind(formula)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTApp {
			family, err := p.manager.GetFamilyID(formula)
			if err != nil {
				return false, err
			}
			op, err := p.manager.GetDeclKind(formula)
			if err != nil {
				return false, err
			}
			if family == p.manager.BasicFamilyID() && op == ast.ASTOpOr {
				n, err := p.manager.GetNumArgs(formula)
				if err != nil {
					return false, err
				}
				clause = true
				args = nil
				for i := uint32(0); i < n; i++ {
					arg, err := p.manager.GetArg(formula, i)
					if err != nil {
						return false, err
					}
					args = append(args, arg)
				}
			}
		}
	}
	if !clause {
		_, _, _, _, matched, err := p.inequality(formula, false)
		return matched, err
	}
	found := false
	for _, arg := range args {
		_, _, literal, err := p.literal(arg)
		if err != nil {
			return false, err
		}
		if literal {
			continue
		}
		_, _, _, _, matched, err := p.inequality(arg, false)
		if err != nil {
			return false, err
		}
		if !matched || found {
			return false, nil
		}
		found = true
	}
	return found, nil
}

func (p *FMInputParser) collectForbidden(goal *tactic.Goal) error {
	visited := map[ast.ASTHandle]bool{}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		formula, err := goal.Form(i)
		if err != nil {
			return err
		}
		occ, err := p.isOCC(formula)
		if err != nil {
			return err
		}
		if occ {
			continue
		}
		if err = p.store.diagnostics.notOCC(formula); err != nil {
			return err
		}
		stack := []ast.ASTHandle{formula}
		for len(stack) != 0 {
			n := len(stack) - 1
			term := stack[n]
			stack = stack[:n]
			if visited[term] {
				continue
			}
			visited[term] = true
			kind, err := p.manager.GetKind(term)
			if err != nil {
				return err
			}
			switch kind {
			case ast.ASTApp:
				constant, err := p.uninterpretedConstant(term)
				if err != nil {
					return err
				}
				if constant {
					sort, err := p.manager.GetSort(term)
					if err != nil {
						return err
					}
					family, err := p.manager.GetFamilyID(sort)
					if err != nil {
						return err
					}
					if family == p.Family {
						if err = p.Forbid(term); err != nil {
							return err
						}
					}
				}
				count, err := p.manager.GetNumArgs(term)
				if err != nil {
					return err
				}
				for j := uint32(0); j < count; j++ {
					child, err := p.manager.GetArg(term, j)
					if err != nil {
						return err
					}
					stack = append(stack, child)
				}
			case ast.ASTQuantifier:
				body, err := p.manager.GetQuantifierBody(term)
				if err != nil {
					return err
				}
				stack = append(stack, body)
				patterns, err := p.manager.GetNumPatterns(term)
				if err != nil {
					return err
				}
				for j := uint32(0); j < patterns; j++ {
					child, err := p.manager.GetPattern(term, j)
					if err != nil {
						return err
					}
					stack = append(stack, child)
				}
				nopatterns, err := p.manager.GetNumNoPatterns(term)
				if err != nil {
					return err
				}
				for j := uint32(0); j < nopatterns; j++ {
					child, err := p.manager.GetNoPattern(term, j)
					if err != nil {
						return err
					}
					stack = append(stack, child)
				}
			}
		}
	}
	return nil
}
