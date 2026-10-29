package macros

// Source: src/ast/macros/macro_util.cpp:393-758,830-930.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (u *Util) QuasiMacroHeadToMacroHead(qhead ast.ASTHandle, declarations uint32, head, condition ast.ASTRef) (uint32, error) {
	n, err := u.manager.GetNumArgs(qhead)
	if err != nil {
		return declarations, err
	}
	found := make([]bool, declarations)
	args := make([]ast.ASTHandle, n)
	conditions := make([]ast.ASTHandle, 0, n)
	next := declarations
	for i := uint32(0); i < n; i++ {
		arg, e := u.manager.GetArg(qhead, i)
		if e != nil {
			return declarations, e
		}
		kind, e := u.manager.GetKind(arg)
		if e != nil {
			return declarations, e
		}
		if kind == ast.ASTVar {
			index, e := u.manager.GetVarIndex(arg)
			if e != nil || index >= declarations {
				return declarations, errors.Join(e, ast.ErrASTDomain)
			}
			if !found[index] {
				found[index], args[i] = true, arg
				continue
			}
		}
		sort, e := u.manager.GetSort(arg)
		if e != nil {
			return declarations, e
		}
		variable, e := u.manager.MkVar(next, sort)
		if e != nil {
			return declarations, e
		}
		next++
		args[i] = variable
		equal, e := u.manager.MkEq(variable, arg)
		if e != nil {
			return declarations, e
		}
		conditions = append(conditions, equal)
	}
	declaration, err := u.manager.GetDecl(qhead)
	if err != nil {
		return declarations, err
	}
	converted, err := u.manager.MkApp(declaration, args)
	if err != nil {
		return declarations, err
	}
	cond, err := u.manager.MkAndArray(conditions)
	if err != nil {
		return declarations, err
	}
	if err = u.manager.SetRef(head, converted); err != nil {
		return declarations, err
	}
	return next, u.manager.SetRef(condition, cond)
}

func (u *Util) hintHead(term ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	kind, err := u.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	declaration, err := u.manager.GetDecl(term)
	if err != nil {
		return nil, false, err
	}
	associative, err := u.manager.RewriterAssociative(declaration)
	if err != nil {
		return nil, false, err
	}
	family, err := u.manager.GetFamilyID(declaration)
	if err != nil || associative || family != ast.NullFamilyID {
		return nil, false, err
	}
	n, err := u.manager.GetNumArgs(term)
	if err != nil {
		return nil, false, err
	}
	variables := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		arg, e := u.manager.GetArg(term, i)
		if e != nil {
			return nil, false, e
		}
		kind, e := u.manager.GetKind(arg)
		if e != nil {
			return nil, false, e
		}
		if kind == ast.ASTVar {
			variables = append(variables, arg)
		}
	}
	return variables, len(variables) != 0, nil
}

func (u *Util) variablesSubset(term ast.ASTHandle, variables []ast.ASTHandle) (bool, error) {
	allowed := make(map[ast.ASTHandle]struct{}, len(variables))
	for _, variable := range variables {
		allowed[variable] = struct{}{}
	}
	seen := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{term}
	for len(stack) != 0 {
		i := len(stack) - 1
		current := stack[i]
		stack = stack[:i]
		if _, yes := seen[current]; yes {
			continue
		}
		seen[current] = struct{}{}
		ground, err := u.manager.IsGround(current)
		if err != nil {
			return false, err
		}
		if ground {
			continue
		}
		kind, err := u.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTVar:
			if _, yes := allowed[current]; !yes {
				return false, nil
			}
		case ast.ASTApp:
			n, e := u.manager.GetNumArgs(current)
			if e != nil {
				return false, e
			}
			for j := n; j > 0; j-- {
				arg, e := u.manager.GetArg(current, j-1)
				if e != nil {
					return false, e
				}
				stack = append(stack, arg)
			}
		default:
			return false, nil
		}
	}
	return true, nil
}

func (u *Util) isHintAtom(head, definition ast.ASTHandle) (bool, error) {
	variables, yes, err := u.hintHead(head)
	if err != nil || !yes {
		return false, err
	}
	declaration, err := u.manager.GetDecl(head)
	if err != nil {
		return false, err
	}
	occurs, err := u.manager.OccursDecl(declaration, definition)
	if err != nil || occurs {
		return false, err
	}
	return u.variablesSubset(definition, variables)
}

func (u *Util) hintToMacroHead(old ast.ASTHandle, declarations uint32, out ast.ASTRef) (uint32, error) {
	n, err := u.manager.GetNumArgs(old)
	if err != nil {
		return declarations, err
	}
	found := make([]bool, declarations)
	args := make([]ast.ASTHandle, n)
	next := declarations
	for i := uint32(0); i < n; i++ {
		arg, e := u.manager.GetArg(old, i)
		if e != nil {
			return declarations, e
		}
		kind, e := u.manager.GetKind(arg)
		if e != nil {
			return declarations, e
		}
		if kind == ast.ASTVar {
			index, e := u.manager.GetVarIndex(arg)
			if e != nil || index >= declarations {
				return declarations, errors.Join(e, ast.ErrASTDomain)
			}
			if !found[index] {
				found[index], args[i] = true, arg
				continue
			}
		}
		sort, e := u.manager.GetSort(arg)
		if e != nil {
			return declarations, e
		}
		args[i], e = u.manager.MkVar(next, sort)
		if e != nil {
			return declarations, e
		}
		next++
	}
	declaration, err := u.manager.GetDecl(old)
	if err != nil {
		return declarations, err
	}
	head, err := u.manager.MkApp(declaration, args)
	if err != nil {
		return declarations, err
	}
	return next, u.manager.SetRef(out, head)
}

func (u *Util) restContains(declaration, except ast.ASTHandle) (bool, error) {
	if u.clause.IsNull() {
		return false, nil
	}
	n, err := u.manager.ClauseNumLiterals(u.clause)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		literal, e := u.manager.ClauseLiteral(u.clause, i)
		if e != nil {
			return false, e
		}
		if literal != except {
			yes, e := u.manager.OccursDecl(declaration, literal)
			if e != nil || yes {
				return yes, e
			}
		}
	}
	return false, nil
}

func (u *Util) restCondition(except ast.ASTHandle) (ast.ASTHandle, error) {
	if u.clause.IsNull() {
		return ast.ASTHandle{}, nil
	}
	n, err := u.manager.ClauseNumLiterals(u.clause)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negated := make([]ast.ASTHandle, 0, n-1)
	for i := uint32(0); i < n; i++ {
		literal, e := u.manager.ClauseLiteral(u.clause, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if literal != except {
			literal, e = u.manager.MkNotSimplified(literal)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			negated = append(negated, literal)
		}
	}
	if len(negated) == 0 {
		return ast.ASTHandle{}, nil
	}
	return u.manager.MkAndArray(negated)
}

func (u *Util) insertCandidate(head ast.ASTHandle, declarations uint32, definition, condition ast.ASTHandle, inequality, satisfy, hint bool, candidates *Candidates) (result error) {
	normalHead, err := u.manager.NewRef(head)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, u.manager.DelRef(&normalHead)) }()
	if yes, e := u.IsMacroHead(head, declarations); e != nil {
		return e
	} else if !yes {
		if hint {
			declarations, err = u.hintToMacroHead(head, declarations, normalHead)
		} else {
			extra, e := u.manager.NewRef(ast.ASTHandle{})
			if e != nil {
				return e
			}
			defer func() { result = errors.Join(result, u.manager.DelRef(&extra)) }()
			declarations, err = u.QuasiMacroHeadToMacroHead(head, declarations, normalHead, extra)
			if err != nil {
				return err
			}
			extraCondition, e := u.manager.RefGet(extra)
			if e != nil {
				return e
			}
			if condition.IsNull() {
				condition = extraCondition
			} else {
				condition, e = u.manager.MkAnd(condition, extraCondition)
				if e != nil {
					return e
				}
			}
		}
	}
	head, err = u.manager.RefGet(normalHead)
	if err != nil {
		return err
	}
	normalDefinition, err := u.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, u.manager.DelRef(&normalDefinition)) }()
	if err = u.NormalizeExpr(head, declarations, definition, normalDefinition); err != nil {
		return err
	}
	definition, err = u.manager.RefGet(normalDefinition)
	if err != nil {
		return err
	}
	if !hint {
		if condition.IsNull() {
			condition, err = u.manager.MkTrue()
			if err != nil {
				return err
			}
		} else {
			normalCondition, e := u.manager.NewRef(ast.ASTHandle{})
			if e != nil {
				return e
			}
			defer func() { result = errors.Join(result, u.manager.DelRef(&normalCondition)) }()
			if e = u.NormalizeExpr(head, declarations, condition, normalCondition); e != nil {
				return e
			}
			condition, e = u.manager.RefGet(normalCondition)
			if e != nil {
				return e
			}
		}
	}
	declaration, err := u.manager.GetDecl(head)
	if err != nil {
		return err
	}
	return candidates.Insert(Candidate{Declaration: declaration, Definition: definition, Condition: condition, Inequality: inequality, SatisfyAtom: satisfy, Hint: hint})
}

func (u *Util) collectCandidateCore(atom ast.ASTHandle, declarations uint32, candidates *Candidates) error {
	family, err := u.manager.GetFamilyID(atom)
	if err != nil {
		return err
	}
	kind, err := u.manager.GetDeclKind(atom)
	if err != nil {
		return err
	}
	if family != u.manager.BasicFamilyID() || kind != ast.ASTOpEq && kind != ast.ASTOpIff {
		return u.collectArithmeticCandidates(atom, declarations, candidates)
	}
	lhs, err := u.manager.GetArg(atom, 0)
	if err != nil {
		return err
	}
	rhs, err := u.manager.GetArg(atom, 1)
	if err != nil {
		return err
	}
	for _, pair := range [][2]ast.ASTHandle{{lhs, rhs}, {rhs, lhs}} {
		head, definition := pair[0], pair[1]
		quasi, e := u.IsQuasiMacroHead(head, declarations)
		if e != nil {
			return e
		}
		if quasi {
			declaration, e := u.manager.GetDecl(head)
			if e != nil {
				return e
			}
			occurs, e := u.manager.OccursDecl(declaration, definition)
			if e != nil {
				return e
			}
			rest, e := u.restContains(declaration, atom)
			if e != nil {
				return e
			}
			if !u.forbiddenDecl(declaration) && !occurs && !rest {
				condition, e := u.restCondition(atom)
				if e != nil {
					return e
				}
				if e = u.insertCandidate(head, declarations, definition, condition, false, true, false, candidates); e != nil {
					return e
				}
				continue
			}
		}
		hint, e := u.isHintAtom(head, definition)
		if e != nil {
			return e
		}
		if hint {
			if e = u.insertCandidate(head, declarations, definition, ast.ASTHandle{}, false, true, true, candidates); e != nil {
				return e
			}
		}
	}
	return u.collectArithmeticCandidates(atom, declarations, candidates)
}

func (u *Util) CollectMacroCandidates(atom ast.ASTHandle, declarations uint32, candidates *Candidates) error {
	if candidates == nil || candidates.manager != u.manager {
		return ast.ErrASTHandle
	}
	if err := candidates.Reset(); err != nil {
		return err
	}
	u.clause = ast.ASTHandle{}
	return u.collectCandidateCore(atom, declarations, candidates)
}

func (u *Util) CollectQuantifierCandidates(q ast.ASTHandle, candidates *Candidates) (result error) {
	if candidates == nil || candidates.manager != u.manager {
		return ast.ErrASTHandle
	}
	if err := candidates.Reset(); err != nil {
		return err
	}
	body, err := u.manager.GetQuantifierBody(q)
	if err != nil {
		return err
	}
	hasQuantifiers, err := u.manager.HasQuantifiers(body)
	if err != nil || hasQuantifiers {
		return err
	}
	declarations, err := u.manager.GetNumDeclarations(q)
	if err != nil {
		return err
	}
	clause, err := u.manager.IsClause(body)
	if err != nil {
		return err
	}
	if !clause {
		u.clause = ast.ASTHandle{}
		return u.collectCandidateCore(body, declarations, candidates)
	}
	u.clause = body
	defer func() { u.clause = ast.ASTHandle{} }()
	n, err := u.manager.ClauseNumLiterals(body)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		literal, e := u.manager.ClauseLiteral(body, i)
		if e != nil {
			return e
		}
		if e = u.collectCandidateCore(literal, declarations, candidates); e != nil {
			return e
		}
	}
	return nil
}
