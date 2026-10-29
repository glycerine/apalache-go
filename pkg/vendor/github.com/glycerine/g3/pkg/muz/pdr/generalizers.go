package pdr

// Source: core_bool_inductive_generalizer in
// src/muz/pdr/pdr_generalizers.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type generalizedCore struct {
	literals   []ast.ASTHandle
	references []ast.ASTRef
	usesLevel  bool
}

func pinGeneralizedCore(manager *ast.ASTManager, literals []ast.ASTHandle, usesLevel bool) (generalizedCore, error) {
	references, err := pinASTHandles(manager, literals)
	if err != nil {
		return generalizedCore{}, err
	}
	return generalizedCore{literals: append([]ast.ASTHandle(nil), literals...), references: references, usesLevel: usesLevel}, nil
}

func closeGeneralizedCores(manager *ast.ASTManager, cores []generalizedCore) (result error) {
	for index := len(cores) - 1; index >= 0; index-- {
		result = errors.Join(result, closeASTRefs(manager, cores[index].references))
	}
	return result
}

func withFarkasCore(transformer *PredicateTransformer, check func() (PropCheckResult, error)) (result PropCheckResult, resultErr error) {
	if transformer == nil || transformer.solver == nil {
		return PropCheckResult{}, ErrPredicateTransformerOwner
	}
	priorFarkas, err := transformer.solver.UseFarkas()
	if err != nil {
		return PropCheckResult{}, err
	}
	priorSubset := transformer.solver.subsetCore
	if err = transformer.solver.SetSubsetBasedCore(false); err != nil {
		return PropCheckResult{}, err
	}
	if err = transformer.solver.SetUseFarkas(true); err != nil {
		return PropCheckResult{}, errors.Join(err, transformer.solver.SetSubsetBasedCore(priorSubset))
	}
	defer func() {
		resultErr = errors.Join(resultErr, transformer.solver.SetUseFarkas(priorFarkas))
		resultErr = errors.Join(resultErr, transformer.solver.SetSubsetBasedCore(priorSubset))
	}()
	return check()
}

// generalizeConvexCore implements the live method1 branch of
// core_convex_hull_generalizer. Each returned core owns its literals across
// later reachability checks.
func generalizeConvexCore(transformer *PredicateTransformer, level uint32, input []ast.ASTHandle, usesLevel, closed bool) (_ []generalizedCore, resultErr error) {
	if transformer == nil {
		return nil, ErrPredicateTransformerOwner
	}
	if len(input) == 0 {
		core, err := pinGeneralizedCore(transformer.manager, input, usesLevel)
		if err != nil {
			return nil, err
		}
		return []generalizedCore{core}, nil
	}
	original, err := transformer.manager.MkAndArray(input)
	if err != nil {
		return nil, err
	}
	originalRef, err := transformer.manager.NewRef(original)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.manager.DelRef(&originalRef)) }()
	frame, err := transformer.Formulas(level, false)
	if err != nil {
		return nil, err
	}
	frameRef, err := transformer.manager.NewRef(frame)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.manager.DelRef(&frameRef)) }()
	conjuncts, err := transformer.manager.FlattenAnd([]ast.ASTHandle{frame})
	if err != nil {
		return nil, err
	}
	builder, err := newConvexClosure(transformer, closed)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, builder.Close()) }()
	result := make([]generalizedCore, 0, len(conjuncts)+1)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeGeneralizedCores(transformer.manager, result))
			result = nil
		}
	}()
	changed := false
	for _, conjunct := range conjuncts {
		negated, x := transformer.manager.MkNot(conjunct)
		if x != nil {
			return nil, x
		}
		state, x := builder.Apply([]ast.ASTHandle{original, negated})
		if x != nil {
			return nil, x
		}
		stateHandle, x := transformer.manager.RefGet(state)
		if x != nil {
			return nil, errors.Join(x, transformer.manager.DelRef(&state))
		}
		check, x := withFarkasCore(transformer, func() (PropCheckResult, error) {
			return transformer.CheckReachable(level, stateHandle, true, false)
		})
		x = errors.Join(x, transformer.manager.DelRef(&state))
		if x != nil {
			return nil, x
		}
		if check.Status != lbool.False {
			continue
		}
		core, x := pinGeneralizedCore(transformer.manager, check.Core, check.AssumesLevel)
		if x != nil {
			return nil, fmt.Errorf("muz/pdr: convex core publication: %w", x)
		}
		result = append(result, core)
		changed = true
	}
	if !closed || !changed {
		core, x := pinGeneralizedCore(transformer.manager, input, usesLevel)
		if x != nil {
			return nil, x
		}
		result = append(result, core)
	}
	return result, nil
}

func generalizeBoolCore(transformer *PredicateTransformer, level uint32, input generalizedCore) (generalizedCore, error) {
	literals, references, usesLevel, err := generalizeBoolInductiveCore(transformer, level, input.literals, input.usesLevel, 0)
	if err != nil {
		return generalizedCore{}, err
	}
	return generalizedCore{literals: literals, references: references, usesLevel: usesLevel}, nil
}

func (c *Context) generalizeBlockingCores(node *ModelNode, literals []ast.ASTHandle, usesLevel bool) ([]generalizedCore, error) {
	initial, err := pinGeneralizedCore(c.manager, literals, usesLevel)
	if err != nil {
		return nil, err
	}
	cores := []generalizedCore{initial}
	applyConvex := func(closed bool) error {
		next := make([]generalizedCore, 0, len(cores))
		for _, core := range cores {
			generated, x := generalizeConvexCore(node.transformer, node.level, core.literals, core.usesLevel, closed)
			if x != nil {
				return errors.Join(x, closeGeneralizedCores(c.manager, next), closeGeneralizedCores(c.manager, cores))
			}
			next = append(next, generated...)
		}
		if x := closeGeneralizedCores(c.manager, cores); x != nil {
			return errors.Join(x, closeGeneralizedCores(c.manager, next))
		}
		cores = next
		return nil
	}
	if c.useConvexClosure {
		if err = applyConvex(true); err != nil {
			return nil, err
		}
	}
	if c.useConvexInterior {
		if err = applyConvex(false); err != nil {
			return nil, err
		}
	}
	if c.useInductive {
		next := make([]generalizedCore, 0, len(cores))
		for _, core := range cores {
			generalized, x := generalizeBoolCore(node.transformer, node.level, core)
			if x != nil {
				return nil, errors.Join(x, closeGeneralizedCores(c.manager, next), closeGeneralizedCores(c.manager, cores))
			}
			next = append(next, generalized)
		}
		if err = closeGeneralizedCores(c.manager, cores); err != nil {
			return nil, errors.Join(err, closeGeneralizedCores(c.manager, next))
		}
		cores = next
	}
	return cores, nil
}

func closeASTRefs(manager *ast.ASTManager, references []ast.ASTRef) (result error) {
	for index := len(references) - 1; index >= 0; index-- {
		result = errors.Join(result, manager.DelRef(&references[index]))
	}
	return result
}

func pinASTHandles(manager *ast.ASTManager, handles []ast.ASTHandle) ([]ast.ASTRef, error) {
	references := make([]ast.ASTRef, 0, len(handles))
	for _, handle := range handles {
		reference, err := manager.NewRef(handle)
		if err != nil {
			return nil, errors.Join(err, closeASTRefs(manager, references))
		}
		references = append(references, reference)
	}
	return references, nil
}

// generalizeBoolInductiveCore drops literals one at a time, retaining a drop
// whenever the remaining cube is still inductive. The returned references own
// the result across later solver checks and must be released by the caller.
func generalizeBoolInductiveCore(transformer *PredicateTransformer, level uint32, input []ast.ASTHandle, usesLevel bool, failureLimit uint32) (_ []ast.ASTHandle, resultReferences []ast.ASTRef, _ bool, resultErr error) {
	if transformer == nil {
		return nil, nil, usesLevel, ErrPredicateTransformerOwner
	}
	if err := transformer.open(); err != nil {
		return nil, nil, usesLevel, err
	}
	core := append([]ast.ASTHandle(nil), input...)
	workspace, err := pinASTHandles(transformer.manager, core)
	if err != nil {
		return nil, nil, usesLevel, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, closeASTRefs(transformer.manager, workspace))
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(transformer.manager, resultReferences))
			resultReferences = nil
		}
	}()
	if len(core) > 1 {
		truth, err := transformer.manager.MkTrue()
		if err != nil {
			return nil, nil, usesLevel, err
		}
		processed := make(map[ast.ASTHandle]bool, len(core))
		var failures uint32
		for index := 0; index < len(core) && len(core) > 1 && (failureLimit == 0 || failures <= failureLimit); {
			literal := core[index]
			core[index] = truth
			inductive, assumesLevel, reduced, err := transformer.CheckInductive(level, core)
			if err != nil {
				return nil, nil, usesLevel, err
			}
			if inductive {
				pinned, err := pinASTHandles(transformer.manager, reduced)
				if err != nil {
					return nil, nil, usesLevel, err
				}
				workspace = append(workspace, pinned...)
				core = append(core[:0], reduced...)
				usesLevel = assumesLevel
				failures = 0
				index = 0
				for index < len(core) && processed[core[index]] {
					index++
				}
				continue
			}
			core[index] = literal
			processed[literal] = true
			failures++
			index++
		}
	}
	resultReferences, err = pinASTHandles(transformer.manager, core)
	if err != nil {
		return nil, nil, usesLevel, err
	}
	return core, resultReferences, usesLevel, nil
}
