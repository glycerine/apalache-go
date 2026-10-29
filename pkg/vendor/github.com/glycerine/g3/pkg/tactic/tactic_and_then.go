package tactic

// Source: src/tactic/tactical.cpp:26-271.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type andThenTactic struct {
	self          *andThenTactic
	first, second Tactic
	references    uint32
	closed        bool
}

func newAndThenTactic(first, second Tactic) (*andThenTactic, error) {
	if first == nil || second == nil {
		return nil, ast.ErrASTHandle
	}
	if err := first.IncRef(); err != nil {
		return nil, err
	}
	if err := second.IncRef(); err != nil {
		_ = first.DecRef()
		return nil, err
	}
	t := &andThenTactic{first: first, second: second}
	t.self = t
	return t, nil
}

func NewAndThenTactic(tactics ...Tactic) (Tactic, error) {
	if len(tactics) == 0 {
		return nil, ast.ErrASTDomain
	}
	if len(tactics) == 1 {
		return tactics[0], nil
	}
	right := tactics[len(tactics)-1]
	for i := len(tactics) - 2; i >= 0; i-- {
		combined, err := newAndThenTactic(tactics[i], right)
		if err != nil {
			return nil, err
		}
		right = combined
	}
	return right, nil
}

func (t *andThenTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *andThenTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}
func (t *andThenTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references != 0 {
		return nil
	}
	t.closed = true
	err := errors.Join(t.first.DecRef(), t.second.DecRef())
	t.first, t.second = nil, nil
	return err
}

func (t *andThenTactic) Apply(g *Goal, out *TacticResult) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	modes, err := readGoalModes(g)
	if err != nil {
		return err
	}
	first := &TacticResult{}
	defer func() { result = errors.Join(result, first.Close()) }()
	if err := t.first.Apply(g, first); err != nil {
		return err
	}
	return composeTacticResults(g, out, first, t.second.Apply, modes)
}

type tacticApplyFunc func(*Goal, *TacticResult) error

type goalModes struct{ models, proofs, cores bool }

func readGoalModes(g *Goal) (goalModes, error) {
	models, err := g.ModelsEnabled()
	if err != nil {
		return goalModes{}, err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return goalModes{}, err
	}
	cores, err := g.UnsatCoresEnabled()
	return goalModes{models: models, proofs: proofs, cores: cores}, err
}

func composeTacticResults(g *Goal, out, first *TacticResult, applySecond tacticApplyFunc, modes goalModes) (result error) {
	var err error
	if len(first.Goals) == 0 {
		return ast.ErrASTDomain
	}
	if len(first.Goals) != 1 {
		return composeMultipleTacticResults(g, out, first, applySecond, modes)
	}
	if first.Goals[0].IsDecided() {
		if err = out.moveGoalsFrom(first); err != nil {
			return err
		}
		if modes.models && first.ModelConverter != nil {
			return out.SetModelConverter(first.ModelConverter)
		}
		return nil
	}
	second := &TacticResult{}
	defer func() { result = errors.Join(result, second.Close()) }()
	if err = applySecond(first.Goals[0], second); err != nil {
		return err
	}
	first.transferBorrowedGoalOwnershipTo(second)
	if err = out.moveGoalsFrom(second); err != nil {
		return err
	}
	if modes.models {
		combined, err := ConcatModels(g.manager.Budget(), first.ModelConverter, second.ModelConverter)
		if err != nil {
			return err
		}
		if err = out.SetModelConverter(combined); err != nil {
			return err
		}
	}
	if modes.proofs {
		combined, err := ConcatProofConverters(g.manager.Budget(), first.ProofConverter, second.ProofConverter)
		if err != nil {
			return err
		}
		if err = out.SetProofConverter(combined); err != nil {
			return err
		}
	}
	if modes.cores {
		combined, err := g.manager.DependencyJoin(first.Core, second.Core)
		if err != nil {
			return err
		}
		if err = out.SetCore(combined); err != nil {
			return err
		}
	}
	return nil
}

func composeMultipleTacticResults(g *Goal, out, first *TacticResult, applySecond tacticApplyFunc, modes goalModes) (result error) {
	var err error
	if err = out.prepare(g.manager); err != nil {
		return err
	}
	modelConverters := make([]ModelConverter, 0, len(first.Goals))
	proofConverters := make([]ProofConverter, 0, len(first.Goals))
	sizes := make([]uint32, 0, len(first.Goals))
	branches := make([]*TacticResult, 0, len(first.Goals))
	defer func() {
		for _, branch := range branches {
			result = errors.Join(result, branch.Close())
		}
	}()
	core := first.Core
	for subgoalIndex, subgoal := range first.Goals {
		branch := &TacticResult{}
		branches = append(branches, branch)
		if err = applySecond(subgoal, branch); err != nil {
			return err
		}
		first.transferBorrowedGoalOwnershipTo(branch)
		if branch.DecidedSat() {
			if err = out.Reset(); err != nil {
				return err
			}
			if err = out.appendMovedGoalsFrom(branch); err != nil {
				return err
			}
			if modes.models {
				if err = publishComposedSATModel(g, out, first.ModelConverter, branch.ModelConverter, uint32(subgoalIndex)); err != nil {
					return err
				}
			}
			return nil
		}
		if branch.DecidedUnsat() {
			if modes.models {
				modelConverters = append(modelConverters, nil)
			}
			if modes.proofs {
				proof, err := branch.Goals[0].Proof(0)
				if err != nil {
					return err
				}
				converter, err := NewStoredProofConverter(g.manager, proof)
				if err != nil {
					return err
				}
				proofConverters = append(proofConverters, converter)
			}
			if modes.models || modes.proofs {
				sizes = append(sizes, 0)
			}
			if modes.cores {
				dependency, err := branch.Goals[0].Dependency(0)
				if err != nil {
					return err
				}
				core, err = g.manager.DependencyJoin(core, dependency)
				if err != nil {
					return err
				}
			}
			continue
		}
		if modes.models {
			modelConverters = append(modelConverters, branch.ModelConverter)
		}
		if modes.proofs {
			proofConverters = append(proofConverters, branch.ProofConverter)
		}
		if modes.models || modes.proofs {
			sizes = append(sizes, uint32(len(branch.Goals)))
		}
		if modes.cores {
			core, err = g.manager.DependencyJoin(core, branch.Core)
			if err != nil {
				return err
			}
		}
		if err = out.appendMovedGoalsFrom(branch); err != nil {
			return err
		}
	}
	if len(out.Goals) == 0 {
		if err = g.ResetAll(); err != nil {
			return err
		}
		var proof ast.ASTHandle
		if modes.proofs {
			defer func() {
				for _, converter := range proofConverters {
					if converter != nil {
						result = errors.Join(result, converter.Close())
					}
				}
			}()
			proofRef, err := g.manager.NewRef(ast.ASTHandle{})
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, g.manager.DelRef(&proofRef)) }()
			if err = ApplyProofConverters(g.manager, first.ProofConverter, proofConverters, proofRef); err != nil {
				return err
			}
			proof, err = g.manager.RefGet(proofRef)
			if err != nil {
				return err
			}
		}
		falsity, err := g.manager.MkFalse()
		if err != nil {
			return err
		}
		if err = g.AssertWithProof(falsity, proof, core); err != nil {
			return err
		}
		return out.AddGoal(g)
	}
	if modes.models {
		combined, err := concatTacticModelSubgoals(g.manager.Budget(), first.ModelConverter, modelConverters, sizes)
		if err != nil {
			return err
		}
		if err = out.SetModelConverter(combined); err != nil {
			return err
		}
	}
	if modes.proofs {
		combined, err := ConcatStarProofConverters(g.manager.Budget(), first.ProofConverter, proofConverters, sizes)
		if err != nil {
			return err
		}
		if err = out.SetProofConverter(combined); err != nil {
			return err
		}
	}
	if modes.cores {
		if err = out.SetCore(core); err != nil {
			return err
		}
	}
	return nil
}

func publishComposedSATModel(g *Goal, out *TacticResult, first, second ModelConverter, index uint32) (result error) {
	md, err := model.NewModel(g.manager)
	if err != nil {
		return err
	}
	ref := &ModelRef{}
	defer func() { result = errors.Join(result, ref.Close()) }()
	if err = ref.Set(md.Core()); err != nil {
		return err
	}
	if second != nil {
		if err = second.ApplyModel(ref, 0); err != nil {
			return err
		}
	}
	if first != nil {
		if err = first.ApplyModel(ref, index); err != nil {
			return err
		}
	}
	value, err := ref.Get()
	if err != nil {
		return err
	}
	stored, err := NewStoredModel(value)
	if err != nil {
		return err
	}
	return out.SetModelConverter(stored)
}

func (t *andThenTactic) each(action func(Tactic) error) error {
	if err := t.open(); err != nil {
		return err
	}
	return errors.Join(action(t.first), action(t.second))
}
func (t *andThenTactic) UpdateParams(p *uparams.ParamsRef) error {
	return t.each(func(c Tactic) error { return c.UpdateParams(p) })
}
func (t *andThenTactic) CollectParamDescrs(p *uparams.ParamDescrs) error {
	return t.each(func(c Tactic) error { return c.CollectParamDescrs(p) })
}
func (t *andThenTactic) CollectStatistics() map[string]float64 {
	result := make(map[string]float64)
	if t.open() != nil {
		return result
	}
	for _, child := range []Tactic{t.first, t.second} {
		for name, value := range child.CollectStatistics() {
			result[name] += value
		}
	}
	return result
}
func (t *andThenTactic) ResetStatistics() error {
	return t.each(func(c Tactic) error { return c.ResetStatistics() })
}
func (t *andThenTactic) Cleanup() error { return t.each(func(c Tactic) error { return c.Cleanup() }) }
func (t *andThenTactic) Reset() error   { return t.each(func(c Tactic) error { return c.Reset() }) }
func (t *andThenTactic) SetLogic(l usymbol.Symbol) error {
	return t.each(func(c Tactic) error { return c.SetLogic(l) })
}
func (t *andThenTactic) SetProgressCallback(callback func(uint32)) error {
	return t.each(func(c Tactic) error { return c.SetProgressCallback(callback) })
}
func (t *andThenTactic) Translate(m *ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	first, err := t.first.Translate(m)
	if err != nil {
		return nil, err
	}
	second, err := t.second.Translate(m)
	if err != nil {
		return nil, err
	}
	return newAndThenTactic(first, second)
}
