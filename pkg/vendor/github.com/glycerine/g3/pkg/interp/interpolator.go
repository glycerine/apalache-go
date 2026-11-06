package interp

// Source checkpoint: src/interp/iz3interp.cpp and src/interp/iz3interp.h
// orchestration helpers independent of proof translation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

// Options contains the string-valued options accepted by the source
// interpolation entry points.
type Options struct{ values map[string]string }

func NewOptions() *Options { return &Options{values: make(map[string]string)} }

func (o *Options) Set(name, value string) {
	if o == nil {
		return
	}
	if o.values == nil {
		o.values = make(map[string]string)
	}
	o.values[name] = value
}

func (o *Options) Apply(base *Base) error {
	if o == nil {
		return nil
	}
	if base == nil {
		return ast.ErrASTHandle
	}
	for name, value := range o.values {
		if err := base.SetOption(name, value); err != nil {
			return err
		}
	}
	return nil
}

type InterpolationBadTreeError struct{}

func (InterpolationBadTreeError) Error() string { return "iz3_bad_tree" }

type IncompletenessError struct{}

func (IncompletenessError) Error() string { return "iz3_incompleteness" }

type InterpolationFailure struct{ Message string }

func (e InterpolationFailure) Error() string { return e.Message }

type InterpolationError struct{}

func (InterpolationError) Error() string {
	return "theory not supported by interpolation or bad proof"
}

// IsLinearParents reports whether the parent-vector representation is a
// sequence. The final root parent is intentionally ignored, as in the source.
func IsLinearParents(parents []int) bool {
	for i := 0; i+1 < len(parents); i++ {
		if parents[i] != i+1 {
			return false
		}
	}
	return true
}

// ProofAssumptions returns the conclusions of asserted leaves in proof-DAG
// traversal order. Shared proof nodes are visited once.
func ProofAssumptions(manager *Manager, proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	seen := make(map[ast.ASTHandle]bool)
	var result []ast.ASTHandle
	var visit func(ast.ASTHandle) error
	visit = func(current ast.ASTHandle) error {
		if seen[current] {
			return nil
		}
		seen[current] = true
		rule, err := manager.ProofRule(current)
		if err != nil {
			return err
		}
		if rule == ast.ASTPrAsserted {
			conclusion, e := manager.Conclusion(current)
			if e != nil {
				return e
			}
			result = append(result, conclusion)
			return nil
		}
		count, err := manager.NumPremises(current)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			premise, e := manager.Premise(current, i)
			if e != nil {
				return e
			}
			if e = visit(premise); e != nil {
				return e
			}
		}
		return nil
	}
	if err := visit(proof); err != nil {
		return nil, err
	}
	return result, nil
}

func hasInterpolationMarker(manager *Manager, memo map[ast.ASTHandle]bool, term ast.ASTHandle) (bool, error) {
	if value, found := memo[term]; found {
		return value, nil
	}
	op, err := manager.Operator(term)
	if err != nil {
		return false, err
	}
	result := op == OperatorInterp
	if op == OperatorAnd {
		count, e := manager.NumArgs(term)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < count && !result; i++ {
			child, e := manager.Arg(term, i)
			if e != nil {
				return false, e
			}
			result, e = hasInterpolationMarker(manager, memo, child)
			if e != nil {
				return false, e
			}
		}
	}
	memo[term] = result
	return result, nil
}

// CollectConjuncts splits conjunctions only along paths containing an
// interpolation marker. Unmarked conjunctions remain whole formulas.
func CollectConjuncts(manager *Manager, tree ast.ASTHandle) ([]ast.ASTHandle, error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	memo := make(map[ast.ASTHandle]bool)
	var result []ast.ASTHandle
	var collect func(ast.ASTHandle) error
	collect = func(term ast.ASTHandle) error {
		has, err := hasInterpolationMarker(manager, memo, term)
		if err != nil {
			return err
		}
		if !has {
			result = append(result, term)
			return nil
		}
		count, err := manager.NumArgs(term)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			child, e := manager.Arg(term, i)
			if e != nil {
				return e
			}
			if e = collect(child); e != nil {
				return e
			}
		}
		return nil
	}
	if err := collect(tree); err != nil {
		return nil, err
	}
	return result, nil
}

func AssertConjuncts(manager *Manager, target solver.Solver, tree ast.ASTHandle) ([]ast.ASTHandle, error) {
	if target == nil {
		return nil, ast.ErrASTHandle
	}
	constraints, err := CollectConjuncts(manager, tree)
	if err != nil {
		return nil, err
	}
	for _, constraint := range constraints {
		if err = target.Assert(constraint); err != nil {
			return nil, err
		}
	}
	return constraints, nil
}

// FrameReducer removes assertion frames absent from a proof. It borrows the
// manager, proof, and formulas supplied to Reduce.
type FrameReducer struct {
	manager       *Manager
	frames        int
	assertionsMap []int
	original      []int
	used          []bool
	ready         bool
}

func NewFrameReducer(manager *Manager) (*FrameReducer, error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	return &FrameReducer{manager: manager}, nil
}

func (r *FrameReducer) Reduce(parts [][]ast.ASTHandle, parents []int, proof ast.ASTHandle) (assertions [][]ast.ASTHandle, reducedParents []int, err error) {
	if r == nil || r.manager == nil || r.manager.open() != nil {
		return nil, nil, ast.ErrASTHandle
	}
	if len(parts) == 0 || (len(parents) != 0 && len(parents) != len(parts)) {
		return nil, nil, ast.ErrASTDomain
	}
	if len(parents) != 0 {
		if _, err = NewScopes(parents); err != nil {
			return nil, nil, err
		}
	}
	r.frames = len(parts)
	r.assertionsMap = nil
	r.original = append(r.original[:0], parents...)
	r.used = make([]bool, r.frames)
	r.ready = false
	frameMap := make(map[ast.ASTHandle]int)
	for frame := range parts {
		for _, formula := range parts[frame] {
			frameMap[formula] = frame
		}
	}
	seen := make(map[ast.ASTHandle]bool)
	var mark func(ast.ASTHandle) error
	mark = func(current ast.ASTHandle) error {
		if seen[current] {
			return nil
		}
		seen[current] = true
		rule, e := r.manager.ProofRule(current)
		if e != nil {
			return e
		}
		if rule == ast.ASTPrAsserted {
			conclusion, e := r.manager.Conclusion(current)
			if e != nil {
				return e
			}
			if frame, found := frameMap[conclusion]; found {
				r.used[frame] = true
			}
			return nil
		}
		count, e := r.manager.NumPremises(current)
		if e != nil {
			return e
		}
		for i := uint32(0); i < count; i++ {
			premise, e := r.manager.Premise(current, i)
			if e != nil {
				return e
			}
			if e = mark(premise); e != nil {
				return e
			}
		}
		return nil
	}
	if err = mark(proof); err != nil {
		return nil, nil, err
	}
	if len(parents) != 0 {
		usedChildren := make([]int, r.frames)
		for frame := 0; frame < r.frames; frame++ {
			if parents[frame] != ScopeTop && (r.used[frame] || usedChildren[frame] != 0) {
				if usedChildren[frame] > 1 {
					r.used[frame] = true
				}
				usedChildren[parents[frame]]++
			}
		}
	}
	backMap := make([]int, r.frames)
	for frame := range parts {
		if r.used[frame] || frame == r.frames-1 {
			assertions = append(assertions, append([]ast.ASTHandle(nil), parts[frame]...))
			r.assertionsMap = append(r.assertionsMap, frame)
			backMap[frame] = len(assertions) - 1
		}
	}
	if len(parents) != 0 {
		reducedParents = make([]int, len(assertions))
		for frame := range assertions {
			parent := parents[r.assertionsMap[frame]]
			for parent != ScopeTop && !r.used[parent] {
				parent = parents[parent]
			}
			if parent == ScopeTop {
				reducedParents[frame] = ScopeTop
			} else {
				reducedParents[frame] = backMap[parent]
			}
		}
	}
	r.ready = true
	return assertions, reducedParents, nil
}

// FixInterpolants restores interpolants for frames removed by Reduce. Every
// returned reference is owned by the caller.
func (r *FrameReducer) FixInterpolants(reduced []ast.ASTHandle) (result []ast.ASTRef, resultErr error) {
	if r == nil || !r.ready || r.manager == nil || r.manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	if len(reduced) != len(r.assertionsMap)-1 {
		return nil, ast.ErrASTDomain
	}
	handles := make([]ast.ASTHandle, r.frames-1)
	truth, err := r.manager.Make(OperatorTrue)
	if err != nil {
		return nil, err
	}
	for i := range handles {
		handles[i] = truth
	}
	for i, value := range reduced {
		handles[r.assertionsMap[i]] = value
	}
	for frame := 0; frame < r.frames-2; frame++ {
		parent := frame + 1
		if len(r.original) != 0 {
			parent = r.original[frame]
		}
		if parent < r.frames-1 && !r.used[parent] {
			handles[parent], err = r.manager.MkAnd(handles[frame], handles[parent])
			if err != nil {
				return nil, err
			}
		}
	}
	defer func() {
		if resultErr != nil {
			for i := len(result) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, r.manager.ast.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	result = make([]ast.ASTRef, 0, len(handles))
	for _, handle := range handles {
		ref, e := r.manager.ast.NewRef(handle)
		if e != nil {
			return nil, e
		}
		result = append(result, ref)
	}
	return result, nil
}
