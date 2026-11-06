package interp

// Source checkpoint: src/interp/iz3interp.cpp/.h public proof, tree, and
// solver orchestration entry points.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// RunConfig supplies the process services that the C++ implementation obtains
// from global solver state. Options may be nil. SolverFactory is only needed
// when arithmetic certificate reconstruction asks Base.IsSAT for a proof or
// dual model; EvaluatorFactory accompanies model-producing factories.
type RunConfig struct {
	Options          *Options
	Profiler         *Profiler
	SolverFactory    solver.Factory
	EvaluatorFactory solver.ConsequenceEvaluatorFactory
}

func configureInterpolationBase(base *Base, config *RunConfig) error {
	if config == nil {
		return nil
	}
	if err := config.Options.Apply(base); err != nil {
		return err
	}
	if config.SolverFactory != nil {
		return base.ConfigureSolver(config.SolverFactory, config.EvaluatorFactory)
	}
	return nil
}

func interpolationTranslationError(err error) error {
	if err == nil {
		return nil
	}
	var unsupported UnsupportedProofError
	var proof ProofError
	if errors.As(err, &unsupported) || errors.As(err, &proof) {
		return InterpolationError{}
	}
	return err
}

// ProofInterpolants computes interpolants for a proof and a parent-vector
// problem. Each returned ASTRef is independently owned by the caller.
func ProofInterpolants(manager *Manager, proof ast.ASTHandle, parts [][]ast.ASTHandle, parents []int, theory []ast.ASTHandle, config *RunConfig) (result []ast.ASTRef, resultErr error) {
	if manager == nil || manager.open() != nil || proof.IsNull() {
		return nil, ast.ErrASTHandle
	}
	if len(parts) == 0 || len(parents) != 0 && len(parents) != len(parts) {
		return nil, ast.ErrASTDomain
	}
	prepProfile, err := startProfile(config, "Interpolation prep")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, prepProfile.Close()) }()
	reducer, err := NewFrameReducer(manager)
	if err != nil {
		return nil, err
	}
	reduced, reducedParents, err := reducer.Reduce(parts, parents, proof)
	if err != nil {
		return nil, err
	}
	if IsLinearParents(reducedParents) {
		reducedParents = nil
	}
	base, err := NewBaseFromParts(manager, reduced, reducedParents, theory)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, base.Close()) }()
	if err = configureInterpolationBase(base, config); err != nil {
		return nil, err
	}
	if err = prepProfile.Stop(); err != nil {
		return nil, err
	}
	translationProfile, err := startProfile(config, "Proof translation")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, translationProfile.Close()) }()
	translation, err := NewFullTranslation(base)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, translation.Close()) }()
	destination, err := NewInterpolationProof(base)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, destination.Close()) }()
	node, err := translation.Translate(proof, destination)
	if err != nil {
		return nil, interpolationTranslationError(err)
	}
	if err = translationProfile.Stop(); err != nil {
		return nil, err
	}
	weak, err := base.WeakMode()
	if err != nil {
		return nil, err
	}
	interpolationProfile, err := startProfile(config, "Proof interpolation")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, interpolationProfile.Close()) }()
	reducedRefs := make([]ast.ASTRef, 0, len(reduced)-1)
	defer func() {
		for i := len(reducedRefs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.ast.DelRef(&reducedRefs[i]))
		}
	}()
	reducedHandles := make([]ast.ASTHandle, 0, len(reduced)-1)
	for frame := 0; frame < len(reduced)-1; frame++ {
		ref, e := destination.Interpolate(node, DownwardScopeRange(frame), weak)
		if e != nil {
			return nil, interpolationTranslationError(e)
		}
		handle, e := manager.ast.RefGet(ref)
		if e != nil {
			_ = manager.ast.DelRef(&ref)
			return nil, e
		}
		quantified, e := translation.Quantify(handle, DownwardScopeRange(frame))
		if e != nil {
			_ = manager.ast.DelRef(&ref)
			return nil, e
		}
		if quantified != handle {
			if e = manager.ast.SetRef(ref, quantified); e != nil {
				_ = manager.ast.DelRef(&ref)
				return nil, e
			}
			handle = quantified
		}
		reducedRefs = append(reducedRefs, ref)
		reducedHandles = append(reducedHandles, handle)
	}
	result, err = reducer.FixInterpolants(reducedHandles)
	if err != nil {
		return nil, err
	}
	if err = interpolationProfile.Stop(); err != nil {
		return nil, err
	}
	return result, nil
}

// ProofInterpolantsFlat is the one-formula-per-frame source overload.
func ProofInterpolantsFlat(manager *Manager, proof ast.ASTHandle, constraints []ast.ASTHandle, parents []int, theory []ast.ASTHandle, config *RunConfig) ([]ast.ASTRef, error) {
	parts := make([][]ast.ASTHandle, len(constraints))
	for i, constraint := range constraints {
		parts[i] = []ast.ASTHandle{constraint}
	}
	return ProofInterpolants(manager, proof, parts, parents, theory, config)
}

// TreeInterpolants computes results in interpolation-marker traversal order.
// Empty constraints are reconstructed from asserted proof leaves.
func TreeInterpolants(manager *Manager, proof ast.ASTHandle, constraints []ast.ASTHandle, tree ast.ASTHandle, config *RunConfig) (result []ast.ASTRef, resultErr error) {
	if manager == nil || manager.open() != nil || proof.IsNull() || tree.IsNull() {
		return nil, ast.ErrASTHandle
	}
	var err error
	if len(constraints) == 0 {
		constraints, err = ProofAssumptions(manager, proof)
		if err != nil {
			return nil, err
		}
	}
	conversion, err := NewBase(manager, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	parts, parents, theory, positions, err := conversion.ToParentsRepresentation(constraints, tree, false)
	err = errors.Join(err, conversion.Close())
	if err != nil {
		return nil, err
	}
	parentRefs, err := ProofInterpolantsFlat(manager, proof, parts, parents, theory, config)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := len(parentRefs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.ast.DelRef(&parentRefs[i]))
		}
	}()
	parentHandles := make([]ast.ASTHandle, len(parentRefs))
	for i := range parentRefs {
		parentHandles[i], err = manager.ast.RefGet(parentRefs[i])
		if err != nil {
			return nil, err
		}
	}
	falsehood, err := manager.Make(OperatorFalse)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			for i := len(result) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, manager.ast.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	for _, position := range positions {
		value := falsehood
		if position >= 0 && position < len(parentHandles) {
			value = parentHandles[position]
		}
		ref, e := manager.ast.NewRef(value)
		if e != nil {
			return nil, e
		}
		result = append(result, ref)
	}
	return result, nil
}

// SolveTree asserts the marked interpolation problem, solves it, and computes
// interpolants on UNSAT. On SAT/UNKNOWN it optionally returns an acquired model.
// Returned ASTRefs and the model are caller-owned.
func SolveTree(manager *Manager, target solver.Solver, tree ast.ASTHandle, acquireModel bool, config *RunConfig) (status lbool.LBool, constraints []ast.ASTHandle, interpolants []ast.ASTRef, mdl *model.ModelCore, err error) {
	if manager == nil || manager.open() != nil || target == nil || tree.IsNull() {
		return lbool.Undef, nil, nil, nil, ast.ErrASTHandle
	}
	constraints, err = AssertConjuncts(manager, target, tree)
	if err != nil {
		return lbool.Undef, nil, nil, nil, err
	}
	solveProfile, profileErr := startProfile(config, "solving")
	if profileErr != nil {
		return lbool.Undef, constraints, nil, nil, profileErr
	}
	status, err = target.CheckSat(nil)
	err = errors.Join(err, solveProfile.Close())
	if err != nil {
		return status, constraints, nil, nil, err
	}
	if status == lbool.False {
		proof, e := target.Proof()
		if e != nil {
			return status, constraints, nil, nil, e
		}
		interpolants, e = TreeInterpolants(manager, proof, constraints, tree, config)
		return status, constraints, interpolants, nil, e
	}
	if acquireModel {
		mdl, err = target.AcquireModel()
	}
	return status, constraints, nil, mdl, err
}

type profileScope struct {
	profiler *Profiler
	name     string
	active   bool
}

func startProfile(config *RunConfig, name string) (*profileScope, error) {
	if config == nil || config.Profiler == nil {
		return &profileScope{}, nil
	}
	if err := config.Profiler.TimerStart(name); err != nil {
		return nil, err
	}
	return &profileScope{profiler: config.Profiler, name: name, active: true}, nil
}

func (s *profileScope) Stop() error {
	if s == nil || !s.active {
		return nil
	}
	s.active = false
	return s.profiler.TimerStop(s.name)
}

func (s *profileScope) Close() error { return s.Stop() }
