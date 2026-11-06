package qe

// Source: src/qe/qe_datatype_plugin.cpp subst_constructor and subst_nonrec.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type datatypeSubstitutionKey struct{ variable, constructor ast.ASTHandle }
type datatypeSubstitution struct {
	refs   []ast.ASTRef
	term   ast.ASTHandle
	fields []ast.ASTHandle
}

func (s *datatypeSubstitution) close(m *ast.ASTManager) error {
	for len(s.refs) != 0 {
		i := len(s.refs) - 1
		if err := m.DelRef(&s.refs[i]); err != nil {
			return err
		}
		s.refs = s.refs[:i]
	}
	return nil
}

func (p *DatatypeSolverPlugin) substitution(variable, constructor ast.ASTHandle) (result *datatypeSubstitution, err error) {
	key := datatypeSubstitutionKey{variable, constructor}
	if cached := p.substitutions[key]; cached != nil {
		for _, field := range cached.fields {
			owner, e := p.manager.NewRef(field)
			if e != nil {
				return nil, e
			}
			if e = p.context.adoptVariable(field, &owner); e != nil {
				return nil, errors.Join(e, p.manager.DelRef(&owner))
			}
		}
		return cached, nil
	}
	entry := &datatypeSubstitution{}
	defer func() {
		if err != nil {
			err = errors.Join(err, entry.close(p.manager))
		}
	}()
	hold := func(h ast.ASTHandle) error {
		ref, e := p.manager.NewRef(h)
		if e == nil {
			entry.refs = append(entry.refs, ref)
		}
		return e
	}
	if err = hold(variable); err != nil {
		return nil, err
	}
	if err = hold(constructor); err != nil {
		return nil, err
	}
	arity, err := p.manager.GetArity(constructor)
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < arity; i++ {
		sort, e := p.manager.GetDomain(constructor, i)
		if e != nil {
			return nil, e
		}
		field, e := p.manager.MkFreshConst("x", sort)
		if e != nil {
			return nil, e
		}
		owner, e := p.manager.NewRef(field)
		if e != nil {
			return nil, e
		}
		if e = p.context.adoptVariable(field, &owner); e != nil {
			return nil, errors.Join(e, p.manager.DelRef(&owner))
		}
		entry.fields = append(entry.fields, field)
	}
	entry.term, err = p.manager.MkApp(constructor, entry.fields)
	if err != nil {
		return nil, err
	}
	if err = hold(entry.term); err != nil {
		return nil, err
	}
	p.substitutions[key] = entry
	return entry, nil
}

// SubstituteNonRecursive replaces the target with the selected constructor.
// Formula and optional definition are caller-owned references; a false result
// leaves recursive datatypes to their not-yet-ported substitution path.
func (p *DatatypeSolverPlugin) SubstituteNonRecursive(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) (supported bool, err error) {
	if err = p.open(); err != nil {
		return false, err
	}
	if variable == nil || value == nil {
		return false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	current, err := p.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	count, supported, err := p.NonRecursiveBranches(target, current)
	if err != nil || !supported {
		return false, err
	}
	unsigned, err := p.manager.NumeralService().IsUnsigned(value)
	if err != nil {
		return false, err
	}
	if !unsigned {
		return false, ast.ErrASTDomain
	}
	branch, err := p.manager.NumeralService().Unsigned(value)
	if err != nil {
		return false, err
	}
	if branch >= count {
		return false, ast.ErrASTDomain
	}
	constructor, found, err := p.findRecognizer(target, current)
	if err != nil {
		return false, err
	}
	if !found {
		sort, e := p.manager.GetSort(target)
		if e != nil {
			return false, e
		}
		constructors, e := p.manager.DatatypeConstructors(sort)
		if e != nil {
			return false, e
		}
		constructor = constructors[branch]
	}
	entry, err := p.substitution(target, constructor)
	if err != nil {
		return false, err
	}
	if definition != nil {
		if err = p.manager.SetRef(*definition, entry.term); err != nil {
			return false, err
		}
	}
	replace, err := rewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	if err = replace.ApplySubstitution(target, entry.term, formula); err != nil {
		return false, err
	}
	return true, nil
}

// SubstituteRecursive covers constructor selection, matching selectors, and
// prepared equality and disequality branches.
func (p *DatatypeSolverPlugin) SubstituteRecursive(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) (supported bool, err error) {
	if err = p.open(); err != nil {
		return false, err
	}
	if variable == nil || value == nil {
		return false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	current, err := p.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	sort, err := p.manager.GetSort(target)
	if err != nil {
		return false, err
	}
	recursive, err := p.manager.DatatypeIsRecursive(sort)
	if err != nil || !recursive {
		return false, err
	}
	unsigned, err := p.manager.NumeralService().IsUnsigned(value)
	if err != nil {
		return false, err
	}
	if !unsigned {
		return false, ast.ErrASTDomain
	}
	branch, err := p.manager.NumeralService().Unsigned(value)
	if err != nil {
		return false, err
	}
	constructor, found, err := p.findRecognizer(target, current)
	if err != nil {
		return false, err
	}
	if found {
		selector, e := p.hasSelector(target, current, constructor)
		if e != nil {
			return false, e
		}
		if !selector {
			return p.substituteEqualityBranch(target, current, constructor, branch, formula, definition)
		}
		if branch != 0 {
			return false, ast.ErrASTDomain
		}
		entry, e := p.substitution(target, constructor)
		if e != nil {
			return false, e
		}
		if definition != nil {
			if e = p.manager.SetRef(*definition, entry.term); e != nil {
				return false, e
			}
		}
		replace, e := rewriter.NewExprSafeReplace(p.manager)
		if e != nil {
			return false, e
		}
		defer func() { err = errors.Join(err, replace.Close()) }()
		if e = replace.ApplySubstitution(target, entry.term, formula); e != nil {
			return false, e
		}
		return true, nil
	}
	constructors, err := p.manager.DatatypeConstructors(sort)
	if err != nil {
		return false, err
	}
	if uint64(branch) >= uint64(len(constructors)) {
		return false, ast.ErrASTDomain
	}
	recognizer, err := p.manager.DatatypeConstructorRecognizer(constructors[branch])
	if err != nil {
		return false, err
	}
	condition, err := p.manager.MkApp(recognizer, []ast.ASTHandle{target})
	if err != nil {
		return false, err
	}
	conditionRef, err := p.manager.NewRef(condition)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&conditionRef)) }()
	conjunction, err := p.manager.MkAnd(condition, current)
	if err != nil {
		return false, err
	}
	if err = p.manager.SetRef(formula, conjunction); err != nil {
		return false, err
	}
	fresh, err := p.manager.MkFreshConst("x", sort)
	if err != nil {
		return false, err
	}
	owner, err := p.manager.NewRef(fresh)
	if err != nil {
		return false, err
	}
	if err = p.context.adoptVariable(fresh, &owner); err != nil {
		return false, errors.Join(err, p.manager.DelRef(&owner))
	}
	replace, err := rewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	if err = replace.ApplySubstitution(target, fresh, formula); err != nil {
		return false, err
	}
	if definition != nil {
		if err = p.manager.SetRef(*definition, fresh); err != nil {
			return false, err
		}
	}
	return true, nil
}
