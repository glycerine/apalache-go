package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::find_arr_eqs.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyArrayProjection struct {
	syntax            *ArrayProjection
	theory            *rewriter.TheoryRewriter
	theoryInterrupted bool
}

type legacyArrayEqualities struct {
	manager    *ast.ASTManager
	equalities []ast.ASTRef
	stores     map[ast.ASTHandle]bool
}

// legacyArrayEqualitiesConstructionFailure preserves find_arr_eqs' local
// result when traversal failed after publication and native rollback refused.
type legacyArrayEqualitiesConstructionFailure struct {
	owner          *legacyArrayEqualities
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyArrayEqualitiesConstructionFailure)(nil)

func (f *legacyArrayEqualitiesConstructionFailure) Error() string                 { return f.reported.Error() }
func (f *legacyArrayEqualitiesConstructionFailure) Unwrap() error                 { return f.reported }
func (f *legacyArrayEqualitiesConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayEqualitiesConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayEqualitiesConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayEqualitiesConstructionFailure) Cleanup() error {
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

func closeFailedLegacyArrayEqualities(owner *legacyArrayEqualities, cause error) error {
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
	return &legacyArrayEqualitiesConstructionFailure{
		owner: owner, nested: nested, runtime: owner.manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported,
	}
}

// legacyArrayProjectionConstructionFailure keeps the actual partially built
// legacy service reachable when constructor rollback is refused.
type legacyArrayProjectionConstructionFailure struct {
	owner          *legacyArrayProjection
	runtime        *memory.Runtime
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*legacyArrayProjectionConstructionFailure)(nil)

func (f *legacyArrayProjectionConstructionFailure) Error() string { return f.Unwrap().Error() }
func (f *legacyArrayProjectionConstructionFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanup)
}
func (f *legacyArrayProjectionConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayProjectionConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayProjectionConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayProjectionConstructionFailure) Cleanup() error {
	if f.owner == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.owner.Close()
	if f.cleanup == nil {
		f.owner = nil
	}
	return f.cleanup
}

func closeFailedLegacyArrayProjection(owner *legacyArrayProjection, cause error) error {
	if cleanup := owner.Close(); cleanup != nil {
		return &legacyArrayProjectionConstructionFailure{owner: owner, runtime: owner.syntax.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup}
	}
	return cause
}

func newLegacyArrayProjection(manager *ast.ASTManager, config Config) (*legacyArrayProjection, error) {
	if _, err := manager.RegisterArrayPlugin(); err != nil {
		return nil, err
	}
	syntax, err := NewArrayProjection(manager, config)
	if err != nil {
		return nil, err
	}
	theory, err := rewriter.NewTheoryRewriter(manager, config.Context, config.Globals, nil)
	if err != nil {
		return nil, closeFailedLegacyArrayProjection(&legacyArrayProjection{syntax: syntax, theory: theory}, err)
	}
	return &legacyArrayProjection{syntax: syntax, theory: theory}, nil
}

func (p *legacyArrayProjection) Close() error {
	if p == nil || p.syntax == nil {
		return ast.ErrASTHandle
	}
	if p.theory != nil {
		if err := p.theory.Close(); err != nil {
			return err
		}
		p.theory = nil
	}
	if err := p.syntax.Close(); err != nil {
		return err
	}
	p.syntax = nil
	return nil
}

func (r *legacyArrayEqualities) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	for len(r.equalities) != 0 {
		index := len(r.equalities) - 1
		if err := r.manager.DelRef(&r.equalities[index]); err != nil {
			return err
		}
		r.equalities = r.equalities[:index]
	}
	r.manager = nil
	r.stores = nil
	return nil
}

func (p *legacyArrayProjection) arrayExpression(expression ast.ASTHandle) (bool, error) {
	sort, err := p.syntax.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	family, err := p.syntax.manager.GetFamilyID(sort)
	return family == p.syntax.family, err
}

func (p *legacyArrayProjection) findEqualities(variable, formula ast.ASTHandle) (_ *legacyArrayEqualities, resultErr error) {
	if p == nil || p.syntax == nil {
		return nil, ast.ErrASTHandle
	}
	result := &legacyArrayEqualities{manager: p.syntax.manager, stores: make(map[ast.ASTHandle]bool)}
	defer func() {
		if resultErr != nil {
			resultErr = closeFailedLegacyArrayEqualities(result, resultErr)
		}
	}()
	done := make(map[ast.ASTHandle]bool)
	var visit func(ast.ASTHandle) error
	visit = func(current ast.ASTHandle) error {
		if done[current] {
			return nil
		}
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			done[current] = true
			return nil
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return err
		}
		arguments := make([]ast.ASTHandle, count)
		argumentsHaveStores := false
		for index := range arguments {
			arguments[index], err = p.syntax.manager.GetArg(current, uint32(index))
			if err != nil {
				return err
			}
			if err = visit(arguments[index]); err != nil {
				return err
			}
			argumentsHaveStores = argumentsHaveStores || result.stores[arguments[index]]
		}
		_, selectTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return err
		}
		storeArguments, storeTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpStore)
		if err != nil {
			return err
		}
		if (!selectTerm && argumentsHaveStores) || (storeTerm && len(storeArguments) != 0 && storeArguments[0] == variable) {
			result.stores[current] = true
		}
		equalityArguments, equality, err := p.syntax.operation(current, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return err
		}
		if equality && len(equalityArguments) == 2 {
			array, err := p.arrayExpression(equalityArguments[0])
			if err != nil {
				return err
			}
			if equalityArguments[0] == variable || equalityArguments[1] == variable || (array && result.stores[current]) {
				reference, err := p.syntax.manager.NewRef(current)
				if err != nil {
					return err
				}
				result.equalities = append(result.equalities, reference)
			}
		}
		done[current] = true
		return nil
	}
	if err := visit(formula); err != nil {
		return nil, err
	}
	return result, nil
}
