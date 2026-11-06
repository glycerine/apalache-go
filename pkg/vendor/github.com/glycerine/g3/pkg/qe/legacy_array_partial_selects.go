package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::factor_selects.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyFactoredPartialEquality struct {
	manager    *ast.ASTManager
	partial    *legacyPartialEquality
	variables  []ast.ASTRef
	equalities []ast.ASTRef
}

type legacyFactoredPartialConstructionFailure struct {
	owner          *legacyFactoredPartialEquality
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyFactoredPartialConstructionFailure)(nil)

func (f *legacyFactoredPartialConstructionFailure) Error() string                 { return f.reported.Error() }
func (f *legacyFactoredPartialConstructionFailure) Unwrap() error                 { return f.reported }
func (f *legacyFactoredPartialConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyFactoredPartialConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyFactoredPartialConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyFactoredPartialConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.owner == nil || f.owner.manager == nil {
		f.owner = nil
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.owner.Close()
	if f.cleanup == nil {
		f.owner = nil
	}
	return f.cleanup
}

func closeFailedLegacyFactoredPartial(owner *legacyFactoredPartialEquality, cause error) error {
	cleanup := owner.Close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	reported := cleanup
	if cause != nil {
		reported = errors.Join(cause, cleanup)
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyFactoredPartialConstructionFailure{
		owner: owner, nested: nested, runtime: owner.manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported,
	}
}

func (r *legacyFactoredPartialEquality) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	if r.partial != nil {
		if err := r.partial.Close(); err != nil {
			return err
		}
		r.partial = nil
	}
	for len(r.equalities) != 0 {
		index := len(r.equalities) - 1
		if r.equalities[index] == (ast.ASTRef{}) {
			r.equalities = r.equalities[:index]
			continue
		}
		if err := r.manager.DelRef(&r.equalities[index]); err != nil {
			return err
		}
		r.equalities = r.equalities[:index]
	}
	for len(r.variables) != 0 {
		index := len(r.variables) - 1
		if r.variables[index] == (ast.ASTRef{}) {
			r.variables = r.variables[:index]
			continue
		}
		if err := r.manager.DelRef(&r.variables[index]); err != nil {
			return err
		}
		r.variables = r.variables[:index]
	}
	r.manager = nil
	return nil
}

func (p *legacyArrayProjection) factorPartialSelects(md *model.ModelCore, evaluator *model.ModelEvaluator, variable ast.ASTHandle, partial *legacyPartialEquality) (result *legacyFactoredPartialEquality, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || evaluator == nil || partial == nil {
		return nil, ast.ErrASTHandle
	}
	result = &legacyFactoredPartialEquality{manager: p.syntax.manager}
	ownedResult := result
	defer func() {
		if resultErr != nil {
			resultErr = closeFailedLegacyFactoredPartial(ownedResult, resultErr)
			result = nil
		}
	}()
	application, err := partial.Application()
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = closeFailedLegacyArrayRewrite(p.syntax.manager, application, resultErr)
	}()
	root, err := p.syntax.manager.RefGet(application)
	if err != nil {
		return nil, err
	}
	type rewrittenNode struct {
		handle    ast.ASTHandle
		hasStores bool
	}
	cache := make(map[ast.ASTHandle]rewrittenNode)
	var rewrite func(ast.ASTHandle) (rewrittenNode, error)
	rewrite = func(current ast.ASTHandle) (rewrittenNode, error) {
		if cached, ok := cache[current]; ok {
			return cached, nil
		}
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil || kind != ast.ASTApp {
			return rewrittenNode{handle: current}, err
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return rewrittenNode{}, err
		}
		children := make([]ast.ASTHandle, count)
		argumentsHaveStores := false
		changed := false
		for index := range children {
			original, err := p.syntax.manager.GetArg(current, uint32(index))
			if err != nil {
				return rewrittenNode{}, err
			}
			child, err := rewrite(original)
			if err != nil {
				return rewrittenNode{}, err
			}
			children[index] = child.handle
			changed = changed || child.handle != original
			argumentsHaveStores = argumentsHaveStores || child.hasStores
		}
		rebuilt := current
		if changed {
			decl, err := p.syntax.manager.GetDecl(current)
			if err != nil {
				return rewrittenNode{}, err
			}
			rebuilt, err = p.syntax.manager.MkApp(decl, children)
			if err != nil {
				return rewrittenNode{}, err
			}
		}
		storeArgs, storeTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpStore)
		if err != nil {
			return rewrittenNode{}, err
		}
		hasStores := argumentsHaveStores || (storeTerm && len(storeArgs) != 0 && storeArgs[0] == variable)
		selectArgs, selectTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return rewrittenNode{}, err
		}
		if selectTerm && len(selectArgs) >= 2 && (selectArgs[0] == variable || argumentsHaveStores) {
			sort, err := p.syntax.manager.GetSort(rebuilt)
			if err != nil {
				return rewrittenNode{}, err
			}
			fresh, err := p.syntax.manager.MkFreshConst("sel", sort)
			if err != nil {
				return rewrittenNode{}, err
			}
			freshRef, err := p.syntax.manager.NewRef(fresh)
			if err != nil {
				return rewrittenNode{}, err
			}
			result.variables = append(result.variables, freshRef)
			value, err := evaluator.EvaluateValue(rebuilt)
			if err != nil {
				return rewrittenNode{}, err
			}
			valueHandle, err := p.syntax.manager.RefGet(value)
			if err == nil {
				var decl ast.ASTHandle
				decl, err = p.syntax.manager.GetDecl(fresh)
				if err == nil {
					err = md.RegisterConstant(decl, valueHandle)
				}
			}
			err = closeFailedLegacyArrayRewrite(p.syntax.manager, value, err)
			if err != nil {
				return rewrittenNode{}, err
			}
			equality, err := p.syntax.manager.MkEq(fresh, rebuilt)
			if err != nil {
				return rewrittenNode{}, err
			}
			equalityRef, err := p.syntax.manager.NewRef(equality)
			if err != nil {
				return rewrittenNode{}, err
			}
			result.equalities = append(result.equalities, equalityRef)
			node := rewrittenNode{handle: fresh}
			cache[current] = node
			return node, nil
		}
		node := rewrittenNode{handle: rebuilt, hasStores: hasStores && !selectTerm}
		cache[current] = node
		return node, nil
	}
	rewritten, err := rewrite(root)
	if err != nil {
		return nil, err
	}
	count, err := p.syntax.manager.GetNumArgs(rewritten.handle)
	if err != nil || count < 2 {
		return nil, errors.Join(err, ast.ErrASTDomain)
	}
	lhs, err := p.syntax.manager.GetArg(rewritten.handle, 0)
	if err != nil {
		return nil, err
	}
	rhs, err := p.syntax.manager.GetArg(rewritten.handle, 1)
	if err != nil {
		return nil, err
	}
	indices := make([]ast.ASTHandle, 0, count-2)
	for index := uint32(2); index < count; index++ {
		handle, err := p.syntax.manager.GetArg(rewritten.handle, index)
		if err != nil {
			return nil, err
		}
		indices = append(indices, handle)
	}
	result.partial, err = newLegacyPartialEquality(p.syntax.manager, p.syntax.family, lhs, rhs, indices)
	if err != nil {
		return nil, err
	}
	return result, nil
}
