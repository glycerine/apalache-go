package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_select_reducer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyArrayFormula struct {
	manager *ast.ASTManager
	formula ast.ASTRef
}

type legacyArrayRewriteConstructionFailure struct {
	manager        *ast.ASTManager
	result         ast.ASTRef
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyArrayRewriteConstructionFailure)(nil)

func (f *legacyArrayRewriteConstructionFailure) Error() string                 { return f.reported.Error() }
func (f *legacyArrayRewriteConstructionFailure) Unwrap() error                 { return f.reported }
func (f *legacyArrayRewriteConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayRewriteConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayRewriteConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayRewriteConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.result == (ast.ASTRef{}) {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.manager.DelRef(&f.result)
	return f.cleanup
}

func closeFailedLegacyArrayRewrite(manager *ast.ASTManager, result ast.ASTRef, cause error) error {
	var cleanup error
	if result != (ast.ASTRef{}) {
		cleanup = manager.DelRef(&result)
	}
	if cleanup == nil {
		return cause
	}
	if result == (ast.ASTRef{}) {
		return errors.Join(cause, cleanup)
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
	return &legacyArrayRewriteConstructionFailure{
		manager: manager, result: result, nested: nested, runtime: manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported,
	}
}

func (r *legacyArrayFormula) Close() error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	err := r.manager.DelRef(&r.formula)
	if err == nil {
		r.manager = nil
	}
	return err
}

func (p *legacyArrayProjection) rewrite(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	// Pin the input before Reset can release an identical cached expression.
	result, err := p.syntax.manager.NewRef(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	// Source mk_result constructs its th_rewriter locally. The Go utility
	// reuses one, so discard an abandoned traversal before starting another.
	if p.theoryInterrupted {
		if err := p.theory.Reset(); err != nil {
			return ast.ASTRef{}, closeFailedLegacyArrayRewrite(p.syntax.manager, result, err)
		}
		p.theoryInterrupted = false
	}
	if err = p.theory.Rewrite(expression, result); err != nil {
		p.theoryInterrupted = true
		return ast.ASTRef{}, closeFailedLegacyArrayRewrite(p.syntax.manager, result, err)
	}
	return result, nil
}

func (p *legacyArrayProjection) reduceSelects(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAll bool) (result *legacyArrayFormula, resultErr error) {
	if p == nil || p.syntax == nil || p.theory == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = closeFailedLegacyArrayRewrite(result.manager, result.formula, resultErr)
			result = nil
		}
	}()
	if !reduceAll && len(variables) == 0 {
		ref, err := p.syntax.manager.NewRef(formula)
		if err != nil {
			return nil, err
		}
		return &legacyArrayFormula{manager: p.syntax.manager, formula: ref}, nil
	}
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = closeFailedLegacyArrayEvaluator(p.syntax.manager, evaluator, resultErr) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	markedVariables := make(map[ast.ASTHandle]bool)
	for _, variable := range variables {
		markedVariables[variable] = true
	}
	type reducedNode struct {
		handle ast.ASTHandle
	}
	cache := make(map[ast.ASTHandle]reducedNode)
	markedStores := make(map[ast.ASTHandle]bool)
	pinned := make([]ast.ASTRef, 0)
	conditions := make([]ast.ASTRef, 0)
	defer func() {
		// Source reset releases pinned rebuilds before index conditions.
		for index := len(pinned) - 1; index >= 0; index-- {
			resultErr = closeFailedLegacyArrayRewrite(p.syntax.manager, pinned[index], resultErr)
		}
		for index := len(conditions) - 1; index >= 0; index-- {
			resultErr = closeFailedLegacyArrayRewrite(p.syntax.manager, conditions[index], resultErr)
		}
	}()
	equalInModel := func(left, right ast.ASTHandle) (_ bool, evalErr error) {
		if left == right {
			return true, nil
		}
		leftValue, err := evaluator.EvaluateValue(left)
		if err != nil {
			return false, err
		}
		defer func() {
			evalErr = closeFailedLegacyArrayRewrite(p.syntax.manager, leftValue, evalErr)
		}()
		rightValue, err := evaluator.EvaluateValue(right)
		if err != nil {
			return false, err
		}
		defer func() {
			evalErr = closeFailedLegacyArrayRewrite(p.syntax.manager, rightValue, evalErr)
		}()
		leftHandle, err := p.syntax.manager.RefGet(leftValue)
		if err != nil {
			return false, err
		}
		rightHandle, err := p.syntax.manager.RefGet(rightValue)
		if err != nil {
			return false, err
		}
		return p.syntax.manager.AreEqual(leftHandle, rightHandle)
	}
	addCondition := func(condition ast.ASTHandle) error {
		rewritten, err := p.rewrite(condition)
		if err != nil {
			return err
		}
		handle, err := p.syntax.manager.RefGet(rewritten)
		if err != nil {
			return closeFailedLegacyArrayRewrite(p.syntax.manager, rewritten, err)
		}
		if !p.syntax.manager.IsTrue(handle) {
			conditions = append(conditions, rewritten)
		} else {
			err = closeFailedLegacyArrayRewrite(p.syntax.manager, rewritten, nil)
		}
		return err
	}
	reduceApp := func(current ast.ASTHandle, children []ast.ASTHandle, argumentsHaveStores, changed bool) (reducedNode, error) {
		rebuilt := current
		if changed {
			decl, err := p.syntax.manager.GetDecl(current)
			if err != nil {
				return reducedNode{}, err
			}
			rebuilt, err = p.syntax.manager.MkApp(decl, children)
			if err != nil {
				return reducedNode{}, err
			}
			owned, err := p.syntax.manager.NewRef(rebuilt)
			if err != nil {
				return reducedNode{}, err
			}
			pinned = append(pinned, owned)
		}
		selectArgs, selectTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return reducedNode{}, err
		}
		reducingSelect := selectTerm && len(selectArgs) == 2 && (reduceAll || markedStores[selectArgs[0]])
		if reducingSelect {
			array, index := selectArgs[0], selectArgs[1]
			for {
				storeArgs, storeTerm, err := p.syntax.operation(array, p.syntax.family, ast.ASTArrayOpStore)
				if err != nil {
					return reducedNode{}, err
				}
				if !storeTerm {
					break
				}
				if len(storeArgs) != 3 {
					return reducedNode{}, ast.ErrASTDomain
				}
				equal, err := equalInModel(storeArgs[1], index)
				if err != nil {
					return reducedNode{}, err
				}
				condition, err := p.syntax.manager.MkEq(storeArgs[1], index)
				if err != nil {
					return reducedNode{}, err
				}
				if equal {
					if err = addCondition(condition); err != nil {
						return reducedNode{}, err
					}
					node := reducedNode{handle: storeArgs[2]}
					cache[current] = node
					return node, nil
				}
				condition, err = p.syntax.manager.MkNot(condition)
				if err != nil {
					return reducedNode{}, err
				}
				if err = addCondition(condition); err != nil {
					return reducedNode{}, err
				}
				array = storeArgs[0]
			}
			// reduce_core returns the existing select when its base is not a store.
			if array != selectArgs[0] {
				rebuilt, err = p.syntax.manager.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, index})
				if err != nil {
					return reducedNode{}, err
				}
				owned, err := p.syntax.manager.NewRef(rebuilt)
				if err != nil {
					return reducedNode{}, err
				}
				pinned = append(pinned, owned)
			}
		}
		_, storeTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpStore)
		if err != nil {
			return reducedNode{}, err
		}
		// Source marks non-select results only. A reduced select inherits any
		// mark already attached to its replacement expression.
		if !reduceAll && !reducingSelect && (argumentsHaveStores || (storeTerm && len(children) != 0 && markedVariables[children[0]])) {
			markedStores[rebuilt] = true
		}
		node := reducedNode{handle: rebuilt}
		cache[current] = node
		return node, nil
	}
	reduced := reducedNode{handle: formula}
	rootKind, err := p.syntax.manager.GetKind(formula)
	if err != nil {
		return nil, err
	}
	var todo []ast.ASTHandle
	if rootKind == ast.ASTApp {
		todo = append(todo, formula)
	}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		size := len(todo)
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return nil, err
		}
		children := make([]ast.ASTHandle, 0, count)
		argumentsHaveStores, changed := false, false
		// Source schedules every uncached application argument in forward
		// order; the LIFO work stack therefore processes the last one first.
		for index := uint32(0); index < count; index++ {
			original, err := p.syntax.manager.GetArg(current, index)
			if err != nil {
				return nil, err
			}
			kind, err := p.syntax.manager.GetKind(original)
			if err != nil {
				return nil, err
			}
			if kind != ast.ASTApp {
				children = append(children, original)
			} else if child, found := cache[original]; found {
				children = append(children, child.handle)
				changed = changed || child.handle != original
				argumentsHaveStores = argumentsHaveStores || reduceAll || markedStores[child.handle]
			} else {
				todo = append(todo, original)
			}
		}
		if len(todo) > size {
			continue
		}
		todo = todo[:len(todo)-1]
		reduced, err = reduceApp(current, children, argumentsHaveStores, changed)
		if err != nil {
			return nil, err
		}
	}
	conjuncts := make([]ast.ASTHandle, 0, len(conditions)+1)
	for _, condition := range conditions {
		handle, err := p.syntax.manager.RefGet(condition)
		if err != nil {
			return nil, err
		}
		conjuncts = append(conjuncts, handle)
	}
	conjuncts = append(conjuncts, reduced.handle)
	combined, err := p.syntax.manager.MkAndArray(conjuncts)
	if err != nil {
		return nil, err
	}
	final, err := p.rewrite(combined)
	if err != nil {
		return nil, err
	}
	result = &legacyArrayFormula{manager: p.syntax.manager, formula: final}
	return result, nil
}
