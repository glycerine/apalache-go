package qe

// Sources: src/muz/spacer/spacer_legacy_mbp.cpp;
// src/muz/spacer/spacer_util.cpp qe_project legacy path.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type LegacyProjection struct {
	closing    bool
	manager    *ast.ASTManager
	config     Config
	arithmetic *legacyArithmeticProjection
	array      *legacyArrayProjection
}

type LegacyProjectionResult struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

type legacyProjectionConstructionFailure struct {
	owner          *LegacyProjection
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*legacyProjectionConstructionFailure)(nil)

func (f *legacyProjectionConstructionFailure) Error() string                 { return f.Unwrap().Error() }
func (f *legacyProjectionConstructionFailure) Unwrap() error                 { return errors.Join(f.cause, f.cleanup) }
func (f *legacyProjectionConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyProjectionConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyProjectionConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyProjectionConstructionFailure) Cleanup() error {
	if f.nested != nil {
		if err := f.nested.Cleanup(); err != nil {
			f.cleanup = err
			return err
		}
		f.nested = nil
	}
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

func closeFailedLegacyProjection(owner *LegacyProjection, cause error) error {
	if cleanup := owner.Close(); cleanup != nil {
		var nested memory.ConstructionFailure
		_ = errors.As(cause, &nested)
		return &legacyProjectionConstructionFailure{owner: owner, nested: nested, runtime: owner.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup}
	}
	return cause
}

// newLegacyModelEvaluator releases a partially constructed evaluator before
// returning a construction error to legacy projection callers.
func newLegacyModelEvaluator(md *model.ModelCore, config model.ModelEvaluatorConfig) (*model.ModelEvaluator, error) {
	evaluator, err := model.NewModelEvaluator(md, config)
	if err != nil && evaluator != nil {
		err = errors.Join(err, evaluator.Close())
		return nil, err
	}
	return evaluator, err
}

func NewLegacyProjection(manager *ast.ASTManager, config Config) (_ *LegacyProjection, resultErr error) {
	arithmetic, err := newLegacyArithmeticProjection(manager, config)
	if err != nil {
		return nil, err
	}
	array, err := newLegacyArrayProjection(manager, config)
	if err != nil {
		return nil, closeFailedLegacyProjection(&LegacyProjection{manager: manager, arithmetic: arithmetic}, err)
	}
	return &LegacyProjection{manager: manager, config: config, arithmetic: arithmetic, array: array}, nil
}

func (p *LegacyProjection) Close() error {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	p.closing = true
	if p.array != nil {
		if err := p.array.Close(); err != nil {
			return err
		}
		p.array = nil
	}
	if p.arithmetic != nil {
		if err := p.arithmetic.Close(); err != nil {
			return err
		}
		p.arithmetic = nil
	}
	p.manager = nil
	return nil
}

func (r *LegacyProjectionResult) Formula() (ast.ASTHandle, error) {
	if r == nil || r.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return r.manager.RefGet(r.formula)
}

func (r *LegacyProjectionResult) Variables() ([]ast.ASTHandle, error) {
	if r == nil || r.manager == nil {
		return nil, ast.ErrASTHandle
	}
	result := make([]ast.ASTHandle, len(r.variables))
	for index := range r.variables {
		var err error
		result[index], err = r.manager.RefGet(r.variables[index])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *LegacyProjectionResult) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	// Refuse cleanup before discarding any roots when manager entry is pending.
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	for len(r.variables) != 0 {
		index := len(r.variables) - 1
		if err := r.manager.DelRef(&r.variables[index]); err != nil {
			return err
		}
		r.variables = r.variables[:index]
	}
	if r.formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.formula); err != nil {
			return err
		}
	}
	r.manager = nil
	return result
}

func (p *LegacyProjection) substituteModelValues(md *model.ModelCore, formula ast.ASTHandle, variables []ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	if len(variables) == 0 {
		return p.manager.NewRef(formula)
	}
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace,
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return ast.ASTRef{}, err
	}
	replace, err := rewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	values := make([]ast.ASTRef, 0, len(variables))
	defer func() {
		for index := len(values) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&values[index]))
		}
	}()
	for _, variable := range variables {
		value, err := evaluator.EvaluateValue(variable)
		if err != nil {
			return ast.ASTRef{}, err
		}
		values = append(values, value)
		handle, err := p.manager.RefGet(value)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = replace.Insert(variable, handle); err != nil {
			return ast.ASTRef{}, err
		}
	}
	result, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = replace.Apply(formula, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}

func (p *LegacyProjection) Project(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects, retainFailures bool) (*LegacyProjectionResult, error) {
	return p.project(md, variables, formula, reduceAllSelects, retainFailures, false)
}

// ProjectNativeArithmetic follows Spacer's qe_project with use_native_mbp:
// Boolean and array stages stay unchanged, and only arithmetic uses native MBP.
// The native arithmetic call forces elimination independently of retainFailures.
func (p *LegacyProjection) ProjectNativeArithmetic(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects, retainFailures bool) (*LegacyProjectionResult, error) {
	return p.project(md, variables, formula, reduceAllSelects, retainFailures, true)
}

func (p *LegacyProjection) project(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects, retainFailures, nativeArithmetic bool) (result *LegacyProjectionResult, resultErr error) {
	if p == nil || p.closing || p.manager == nil || md == nil || md.Manager() != p.manager {
		return nil, ast.ErrASTHandle
	}
	result = &LegacyProjectionResult{manager: p.manager}
	// Keep the allocation reachable even when an explicit nil return overwrites
	// the named result, including failures after publishing the formula below.
	ownedResult := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, ownedResult.Close())
			result = nil
		}
	}()
	current, err := p.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&current)) }()
	// qe_project flattens its input once before the repeated qe_lite stages.
	// flatten_and's swap/reprocessing order also applies to an already flat
	// conjunction, and must precede any theory rewriting or variable selection.
	flat, err := p.manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return nil, err
	}
	if len(flat) != 0 {
		flattened, err := p.manager.MkAndArray(flat)
		if err != nil {
			return nil, err
		}
		if err = p.manager.SetRef(current, flattened); err != nil {
			return nil, err
		}
	}
	variableRefs := make([]ast.ASTRef, 0, len(variables))
	defer func() { resultErr = errors.Join(resultErr, truncateLiteVector(p.manager, &variableRefs, 0)) }()
	for _, variable := range variables {
		variableRef, err := p.manager.NewRef(variable)
		if err != nil {
			return nil, err
		}
		variableRefs = append(variableRefs, variableRef)
	}
	pending := append([]ast.ASTHandle(nil), variables...)
	ownedVariables := make([]ast.ASTRef, 0)
	defer func() {
		for index := len(ownedVariables) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&ownedVariables[index]))
		}
	}()
	arithmetic := make([]ast.ASTHandle, 0)
	booleanSort, err := p.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	for {
		prepared, err := p.prepareProjectionRound(pending, current)
		if err != nil {
			return nil, fmt.Errorf("projection preparation: %w", err)
		}
		ownedVariables = append(ownedVariables, prepared...)
		pending = pending[:0]
		for _, ref := range prepared {
			handle, err := p.manager.RefGet(ref)
			if err != nil {
				return nil, err
			}
			pending = append(pending, handle)
		}

		if err = p.traceProjectionStage(515, "After qe_lite:", current, "Vars:", pending, nil); err != nil {
			return nil, err
		}

		booleans := make([]ast.ASTHandle, 0)
		arrays := make([]ast.ASTHandle, 0)
		for _, variable := range pending {
			sort, err := p.manager.GetSort(variable)
			if err != nil {
				return nil, err
			}
			if sort == booleanSort {
				booleans = append(booleans, variable)
				continue
			}
			family, err := p.manager.GetFamilyID(sort)
			if err != nil {
				return nil, err
			}
			if family == p.array.syntax.family {
				arrays = append(arrays, variable)
			} else {
				arithmetic = append(arithmetic, variable)
			}
		}
		if len(booleans) != 0 {
			handle, err := p.manager.RefGet(current)
			if err != nil {
				return nil, err
			}
			next, err := p.substituteModelValues(md, handle, booleans)
			if err != nil {
				return nil, fmt.Errorf("Boolean model substitution: %w", err)
			}
			if err = p.manager.DelRef(&current); err != nil {
				_ = p.manager.DelRef(&next)
				return nil, err
			}
			current = next
			if err = p.rewriteProjectionCurrent(current); err != nil {
				return nil, err
			}
		}
		if len(booleans) != 0 {
			if err = p.traceProjectionStage(548, "Projected Booleans:", current, "", nil, nil); err != nil {
				return nil, err
			}
		}
		if err = p.traceProjectionStage(554, "", ast.ASTRef{}, "Array vars:", arrays, nil); err != nil {
			return nil, err
		}
		handle, err := p.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		projected, err := p.projectArrayRound(md, arrays, handle, reduceAllSelects)
		if err != nil {
			return nil, fmt.Errorf("array projection: %w", err)
		}
		nextHandle, err := p.manager.RefGet(projected.formula)
		if err == nil {
			err = p.manager.SetRef(current, nextHandle)
		}
		pending = pending[:0]
		for _, variableRef := range projected.variables {
			if err != nil {
				break
			}
			var handle ast.ASTHandle
			handle, err = p.manager.RefGet(variableRef)
			if err == nil {
				var owned ast.ASTRef
				owned, err = p.manager.NewRef(handle)
				if err == nil {
					ownedVariables = append(ownedVariables, owned)
				}
			}
			if err == nil {
				pending = append(pending, handle)
			}
		}
		err = closeTransferredLegacyOuterArray(projected, err)
		if err != nil {
			return nil, err
		}
		if err = p.traceProjectionStage(574, "", ast.ASTRef{}, "Auxiliary variables of index and value sorts:", pending, md); err != nil {
			return nil, err
		}
		if len(pending) == 0 {
			break
		}
	}
	if len(arithmetic) != 0 {
		if err = p.traceProjectionStage(588, "", ast.ASTRef{}, "Arith vars:", arithmetic, nil); err != nil {
			return nil, err
		}
		handle, err := p.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		var remaining []ast.ASTHandle
		if nativeArithmetic {
			if err = p.projectNativeArithmetic(md, arithmetic, current); err != nil {
				return nil, err
			}
		} else {
			projected, err := p.projectArithmeticRound(md, arithmetic, handle)
			if err != nil {
				return nil, err
			}
			nextHandle, err := p.manager.RefGet(projected.formula)
			if err == nil {
				err = p.manager.SetRef(current, nextHandle)
			}
			remaining = make([]ast.ASTHandle, 0, len(projected.variables))
			for _, variableRef := range projected.variables {
				if err != nil {
					break
				}
				var handle ast.ASTHandle
				handle, err = p.manager.RefGet(variableRef)
				if err == nil {
					remaining = append(remaining, handle)
				}
			}
			err = closeTransferredLegacyOuterArithmetic(projected, err)
			if err != nil {
				return nil, err
			}
		}
		if err = p.traceProjectionStage(615, "Projected arith vars:", current, "Remaining arith vars:", remaining, nil); err != nil {
			return nil, err
		}
		if len(remaining) != 0 {
			remaining, err = p.projectModelTerms(md, current, remaining)
			if err != nil {
				return nil, err
			}
		}
		if len(remaining) != 0 && !retainFailures {
			handle, err := p.manager.RefGet(current)
			if err != nil {
				return nil, err
			}
			next, err := p.substituteModelValues(md, handle, remaining)
			if err != nil {
				return nil, err
			}
			if err = p.manager.DelRef(&current); err != nil {
				_ = p.manager.DelRef(&next)
				return nil, err
			}
			current = next
			if err = p.traceProjectionStage(632, "After substituting remaining arith vars:", current, "", nil, nil); err != nil {
				return nil, err
			}
			if err = p.rewriteProjectionCurrent(current); err != nil {
				return nil, fmt.Errorf("projection substitution rewrite: %w", err)
			}
		} else {
			for _, variable := range remaining {
				variableRef, err := p.manager.NewRef(variable)
				if err != nil {
					return nil, err
				}
				result.variables = append(result.variables, variableRef)
			}
		}
	}
	// Source qe_project rewrites here only after substituting remaining
	// arithmetic variables. MBQI symbolic replacements are published directly.
	result.formula, err = p.manager.CopyRef(current)
	if err != nil {
		return nil, err
	}
	if ast.ASTSourceDebug {
		handle, err := p.manager.RefGet(result.formula)
		if err != nil {
			return nil, err
		}
		holds, err := p.projectionModelHolds(md, handle)
		if err != nil {
			return nil, err
		}
		if !holds {
			return nil, fmt.Errorf("spacer qe_project model sanity: %w", ast.ErrASTDomain)
		}
	}
	return result, nil
}
