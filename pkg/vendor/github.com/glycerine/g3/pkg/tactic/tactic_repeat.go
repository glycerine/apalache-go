package tactic

// Source: src/tactic/tactical.cpp:927-1075.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func moveFirstTacticResultWithMetadata(g *Goal, out, source *TacticResult, modes goalModes) error {
	if len(source.Goals) == 0 {
		return ast.ErrASTDomain
	}
	if err := out.prepare(source.manager); err != nil {
		return err
	}
	out.Goals = append(out.Goals, source.Goals[0])
	out.goalOwned = append(out.goalOwned, source.goalOwned[0])
	source.Goals[0] = nil
	source.goalOwned[0] = false
	source.Goals = source.Goals[1:]
	source.goalOwned = source.goalOwned[1:]
	if modes.models && source.ModelConverter != nil {
		if err := out.SetModelConverter(source.ModelConverter); err != nil {
			return err
		}
	}
	if modes.proofs && source.ProofConverter != nil {
		if err := out.SetProofConverter(source.ProofConverter); err != nil {
			return err
		}
	}
	if modes.cores && !source.Core.IsNull() {
		if err := out.SetCore(source.Core); err != nil {
			return err
		}
	}
	return nil
}

func (t *unaryTactic) repeatAt(depth uint64, g *Goal, out *TacticResult) (result error) {
	if depth > uint64(t.threshold) {
		if err := out.prepare(g.manager); err != nil {
			return err
		}
		return out.AddGoal(g)
	}
	modes, err := readGoalModes(g)
	if err != nil {
		return err
	}
	snapshot, err := NewGoalConfigCopy(g)
	if err != nil {
		return err
	}
	if err = g.CopyTo(snapshot); err != nil {
		return errors.Join(err, snapshot.Close())
	}
	first := &TacticResult{}
	defer func() { result = errors.Join(result, first.Close()) }()
	if err = t.child.Apply(g, first); err != nil {
		return errors.Join(err, snapshot.Close())
	}
	equal, err := GoalsEqual(snapshot, g)
	if closeErr := snapshot.Close(); err == nil {
		err = closeErr
	} else if closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		return err
	}
	if equal {
		return moveFirstTacticResultWithMetadata(g, out, first, modes)
	}
	if len(first.Goals) == 0 {
		return ast.ErrASTDomain
	}
	if len(first.Goals) != 1 {
		return composeMultipleTacticResults(g, out, first, func(next *Goal, target *TacticResult) error {
			return t.repeatAt(depth+1, next, target)
		}, modes)
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
	if err = t.repeatAt(depth+1, first.Goals[0], second); err != nil {
		return err
	}
	first.transferBorrowedGoalOwnershipTo(second)
	if err = out.moveGoalsFrom(second); err != nil {
		return err
	}
	if modes.models {
		combined, err := ConcatModels(g.manager.Budget(), second.ModelConverter, first.ModelConverter)
		if err != nil {
			return err
		}
		if err = out.SetModelConverter(combined); err != nil {
			return err
		}
	}
	if modes.proofs {
		combined, err := ConcatProofConverters(g.manager.Budget(), second.ProofConverter, first.ProofConverter)
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
