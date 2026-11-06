package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::level_replacer.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// ReplacePredicates maps registered predicate applications to the preceding
// level, or false at level zero. Quantifier annotations follow the source
// rewriter's no-pattern behavior.
func (c *Compiler) ReplacePredicates(rules *base.RuleManager, expression ast.ASTHandle, level uint32) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var refs []ast.ASTRef
	defer func() {
		resultErr = disposeASTRefs(m, refs, resultErr)
		if resultErr != nil && result != (ast.ASTRef{}) {
			owned := result
			result = ast.ASTRef{}
			resultErr = disposeASTRefs(m, []ast.ASTRef{owned}, resultErr)
		}
	}()
	hold := func(h ast.ASTHandle) error {
		ref, err := m.NewRef(h)
		if err == nil {
			refs = append(refs, ref)
		}
		return err
	}
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(current ast.ASTHandle) (ast.ASTHandle, error) {
		if value, ok := memo[current]; ok {
			return value, nil
		}
		kind, err := m.GetKind(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		value := current
		switch kind {
		case ast.ASTVar:
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			rewritten, err := visit(body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			ground, err := m.IsGround(rewritten)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if ground {
				value = rewritten
			} else {
				value, err = m.UpdateQuantifierAnnotations(current, nil, []ast.ASTHandle{rewritten}, rewritten)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		case ast.ASTApp:
			count, err := m.GetNumArgs(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args := make([]ast.ASTHandle, count)
			changed := false
			for i := range args {
				arg, e := m.GetArg(current, uint32(i))
				if e != nil {
					return ast.ASTHandle{}, e
				}
				args[i], e = visit(arg)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				changed = changed || args[i] != arg
			}
			decl, err := m.GetDecl(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			predicate, err := rules.IsRegisteredPredicate(decl)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if predicate {
				if level == 0 {
					value, err = m.MkFalse()
				} else {
					levelRef, e := c.levelDeclaration(decl, level-1, nil)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					refs = append(refs, levelRef)
					levelDecl, e := m.RefGet(levelRef)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					value, err = m.MkApp(levelDecl, args)
				}
			} else if changed {
				value, err = m.MkApp(decl, args)
			}
			if err != nil {
				return ast.ASTHandle{}, err
			}
		default:
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if err = hold(value); err != nil {
			return ast.ASTHandle{}, err
		}
		memo[current] = value
		return value, nil
	}
	value, err := visit(expression)
	if err != nil {
		return result, err
	}
	return m.NewRef(value)
}
