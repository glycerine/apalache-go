package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrDerivationRestart = errors.New("duality derivation restart")
var ErrDerivationStackSync = errors.New("duality solver and expansion stacks out of sync")

// DerivationSearch borrows search/inference services. Outgoing unwinding edges
// must already be resolved; stratified GetNodeOutgoing dispatch remains separate.
// Source: duality_solver.cpp:2189-2205,2234-2401.
type DerivationSearch struct {
	Search                                        *BacktrackingDerivation
	Checker                                       *LocalEdgeChecker
	Auxiliary                                     solver.Solver
	Simplifier                                    *interp.Manager
	Cases                                         *CaseInterpolator
	Config                                        *interp.RunConfig
	EnableRestarts                                bool
	restartInterval, totalUpdates, updateFailures int
	wasSAT                                        bool
}

func (s *DerivationSearch) resetMappedFacts(children bool) error {
	d := s.Search
	for _, node := range d.stack[len(d.stack)-1].expansions {
		if node == nil || node.Map == nil {
			return ErrWrapperOwner
		}
		if err := node.Map.Annotation.SetFull(); err != nil {
			return err
		}
		if children {
			if node.Map.Outgoing == nil {
				return ErrWrapperOwner
			}
			for _, child := range node.Map.Outgoing.Children {
				if err := child.Annotation.SetFull(); err != nil {
					return err
				}
			}
		}
		message := "incompleteness: cleared annotation"
		if children {
			message = "incompleteness: cleared annotation and child annotations"
		}
		if err := d.original.streamReporter.Message(message); err != nil {
			return err
		}
	}
	return nil
}
func onlyLearningRestartCauses(err error) bool {
	if err == ErrCaseInterpolationReallyBad || err == ErrCaseInterpolationBad || err == ErrCoreNotUnsat {
		return true
	}
	switch err.(type) {
	case interp.Error, *interp.Error:
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !onlyLearningRestartCauses(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return onlyLearningRestartCauses(wrapped)
	}
	return false
}

func (s *DerivationSearch) learningFailure(err error) error {
	if !onlyLearningRestartCauses(err) {
		return err
	}
	if errors.Is(err, ErrCaseInterpolationReallyBad) {
		if reset := s.resetMappedFacts(true); reset != nil {
			return errors.Join(err, reset)
		}
		return errors.Join(ErrDerivationRestart, err)
	}
	if errors.Is(err, ErrCaseInterpolationBad) {
		return errors.Join(ErrDerivationRestart, err)
	}
	var interpolation interp.Error
	var interpolationPointer *interp.Error
	if errors.Is(err, ErrCoreNotUnsat) || errors.As(err, &interpolation) || errors.As(err, &interpolationPointer) {
		if report := s.Search.original.streamReporter.Message("interpolation failure:" + err.Error()); report != nil {
			return errors.Join(err, report)
		}
		return errors.Join(ErrDerivationRestart, err)
	}
	return err
}
func (s *DerivationSearch) learningProgress(updates int) error {
	s.totalUpdates += updates
	if s.EnableRestarts && s.totalUpdates >= s.restartInterval {
		return ErrDerivationRestart
	}
	if updates != 0 {
		s.updateFailures = 0
		return nil
	}
	if s.wasSAT {
		s.updateFailures++
		if s.updateFailures > 10 {
			if err := s.resetMappedFacts(false); err != nil {
				return err
			}
			return ErrDerivationRestart
		}
	}
	return s.Search.original.streamReporter.Message("backtracked without learning")
}
func (s *DerivationSearch) restart() error {
	d := s.Search
	// Clear before deleting expansion children, preserving the source invariant.
	d.updatedNodes = nil
	for len(d.stack) > 1 {
		if err := d.PopLevel(); err != nil {
			return err
		}
	}
	s.restartInterval++
	return d.original.streamReporter.Message("restarted")
}

// Build runs the source slow-search loop on resolved unwinding edges. True
// preserves a completed counterexample graph; false publishes a refutation.
func (s *DerivationSearch) Build() (bool, error) {
	if s == nil || s.Search == nil || s.Search.DerivationTree == nil || s.Checker == nil || s.Checker.closed || s.Auxiliary == nil || s.Simplifier == nil {
		return false, ErrWrapperOwner
	}
	d := s.Search
	if d.Top == nil || d.Top.Owner != d.tree || len(d.stack) == 0 {
		return false, ErrWrapperOwner
	}
	if err := d.tree.solverReady(); err != nil {
		return false, err
	}
	s.restartInterval = 3
	for {
		result, err := s.buildMain()
		if !errors.Is(err, ErrDerivationRestart) {
			return result, err
		}
		if err = s.restart(); err != nil {
			return false, err
		}
	}
}
func (s *DerivationSearch) buildMain() (bool, error) {
	d := s.Search
	level, err := d.tree.solver.ScopeLevel()
	if err != nil {
		return false, err
	}
	d.stack[len(d.stack)-1].level = int(level)
	s.wasSAT = true
	s.totalUpdates = 0
	s.updateFailures = 0
	for {
		if err := d.tree.checkpoint(); err != nil {
			return false, err
		}
		level, err := d.tree.solver.ScopeLevel()
		if err != nil {
			return false, err
		}
		if int(level) != d.stack[len(d.stack)-1].level {
			return false, ErrDerivationStackSync
		}
		d.original.streamReporter.Depth(len(d.stack))
		status, err := d.tree.Check(d.Top, nil, nil)
		if err != nil {
			return false, err
		}
		if status == lbool.False {
			updates, err := d.LearnCurrentExpansions(s.Checker, s.Auxiliary, s.Simplifier, s.Cases, s.Config)
			if err != nil {
				return false, s.learningFailure(err)
			}
			if err = s.learningProgress(updates); err != nil {
				return false, err
			}
			done, err := d.BacktrackUnsatisfiable(s.Checker, s.Auxiliary)
			if err != nil {
				return false, err
			}
			if done {
				return false, nil
			}
			s.wasSAT = false
		} else {
			s.wasSAT = true
			expanded, err := d.AdvanceSatisfiable()
			if err != nil {
				return false, err
			}
			if !expanded {
				return true, nil
			}
		}
	}
}
