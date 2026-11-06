package base

// Source: dl_context.cpp:1257-1335 is_query and declare_vars.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (s *RuleSet) declareContextVariables(out io.Writer, printer *ast.LegacySMTPrinter, rules []ast.ASTRef, fresh *model.FreshName) (resultErr error) {
	m := s.manager.manager
	owner := &contextSMT2Owner{manager: m}
	defer func() { resultErr = owner.finish(resultErr) }()
	var err error
	owner.substitution, err = rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return err
	}
	bySort := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, r := range rules {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			continue
		}
		forall, err := m.IsForall(h)
		if err != nil {
			return err
		}
		if !forall {
			continue
		}
		body, err := m.GetQuantifierBody(h)
		if err != nil {
			return err
		}
		nested, err := m.HasQuantifiers(body)
		if err != nil {
			return err
		}
		if nested {
			continue
		}
		count, err := m.GetNumDeclarations(h)
		if err != nil {
			return err
		}
		used := make(map[ast.ASTHandle]int)
		args := make([]ast.ASTHandle, count)
		for j := uint32(0); j < count; j++ {
			sort, err := m.GetDeclarationSort(h, count-1-j)
			if err != nil {
				return err
			}
			index := used[sort]
			used[sort] = index + 1
			pool := bySort[sort]
			if index == len(pool) {
				name, err := fresh.Next()
				if err != nil {
					return err
				}
				declarationRef, err := owner.reserve()
				if err != nil {
					return err
				}
				ref, err := owner.reserve()
				if err != nil {
					return err
				}
				declaration, err := m.MkFuncDecl(name, nil, sort, ast.ASTFuncDeclInfo{})
				if !declaration.IsNull() {
					err = errors.Join(err, m.SetRef(declarationRef, declaration))
				}
				if err != nil {
					return err
				}
				constant, err := m.MkApp(declaration, nil)
				if !constant.IsNull() {
					err = errors.Join(err, m.SetRef(ref, constant))
				}
				if err != nil {
					return err
				}
				pool = append(pool, constant)
				bySort[sort] = pool
				if _, err = io.WriteString(out, "(declare-var "); err != nil {
					return err
				}
				text, err := m.Symbols.String(name)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(out, text+" "); err != nil {
					return err
				}
				if err = s.writeContextSort(out, sort); err != nil {
					return err
				}
				if _, err = io.WriteString(out, ")\n"); err != nil {
					return err
				}
			}
			args[j] = pool[index]
		}
		result, err := owner.reserve()
		if err != nil {
			return err
		}
		if err = owner.substitution.Substitute(body, args, result); err != nil {
			return err
		}
		value, err := m.RefGet(result)
		if err != nil {
			return err
		}
		if err = m.SetRef(r, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *RuleSet) isContextQuery(h ast.ASTHandle) (bool, error) {
	m := s.manager.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	d, err := m.GetDecl(h)
	if err != nil {
		return false, err
	}
	if _, yes := s.manager.predicates[d]; !yes {
		return false, nil
	}
	count, err := m.GetNumArgs(h)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		a, e := m.GetArg(h, i)
		if e != nil {
			return false, e
		}
		k, e := m.GetKind(a)
		if e != nil {
			return false, e
		}
		if k != ast.ASTVar {
			return false, nil
		}
		index, e := m.GetVarIndex(a)
		if e != nil {
			return false, e
		}
		if index != i {
			return false, nil
		}
	}
	return true, nil
}

func (s *RuleSet) writeContextQuery(out io.Writer, printer *ast.LegacySMTPrinter, h ast.ASTHandle, low bool) (resultErr error) {
	m := s.manager.manager
	direct, err := s.isContextQuery(h)
	if err != nil {
		return err
	}
	var declaration ast.ASTHandle
	owner := &contextSMT2Owner{manager: m}
	defer func() { resultErr = owner.finish(resultErr) }()
	if direct {
		declaration, err = m.GetDecl(h)
		if err != nil {
			return err
		}
	} else {
		vars, err := ast.NewASTUsedVars(m)
		if err != nil {
			return err
		}
		if err = vars.Collect(h); err != nil {
			return err
		}
		count := vars.MaxFoundVarIndexPlusOne()
		domain, args := make([]ast.ASTHandle, count), make([]ast.ASTHandle, count)
		boolean, err := m.MkBoolSort()
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			sort := vars.Contains(i)
			if sort.IsNull() {
				sort = boolean
			}
			domain[i] = sort
		}
		prefix, err := m.Symbols.Intern("q")
		if err != nil {
			return err
		}
		suffix, err := m.Symbols.Intern("")
		if err != nil {
			return err
		}
		declarationRef, err := owner.reserve()
		if err != nil {
			return err
		}
		declaration, err = m.MkFreshFuncDecl(prefix, suffix, domain, boolean)
		if !declaration.IsNull() {
			err = errors.Join(err, m.SetRef(declarationRef, declaration))
		}
		if err != nil {
			return err
		}
		if err = s.writeContextRelation(out, printer, declaration); err != nil {
			return err
		}
		// Source emits the fresh declaration before constructing the bridge
		// arguments, including Boolean defaults for unused variable indices.
		for i := uint32(0); i < count; i++ {
			ref, err := owner.reserve()
			if err != nil {
				return err
			}
			args[i], err = m.MkVar(i, domain[i])
			if !args[i].IsNull() {
				err = errors.Join(err, m.SetRef(ref, args[i]))
			}
			if err != nil {
				return err
			}
		}
		appRef, err := owner.reserve()
		if err != nil {
			return err
		}
		app, err := m.MkApp(declaration, args)
		if !app.IsNull() {
			err = errors.Join(err, m.SetRef(appRef, app))
		}
		if err != nil {
			return err
		}
		implicationRef, err := owner.reserve()
		if err != nil {
			return err
		}
		implication, err := m.MkImplies(h, app)
		if !implication.IsNull() {
			err = errors.Join(err, m.SetRef(implicationRef, implication))
		}
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, "(assert "); err != nil {
			return err
		}
		if err = s.writeContextExpression(out, printer, implication, low); err != nil {
			return err
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	name, err := m.GetASTName(declaration)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "(query "); err != nil {
		return err
	}
	if err = s.writeContextName(out, name); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")\n")
	return err
}
