package spacer

// Source: src/muz/spacer/spacer_context.h reach_fact.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

// reachFactConstructionFailure preserves a partially destroyed reach fact
// when exceptional constructor cleanup is refused. Close advances through
// justifications, AST roots, and the retained rule independently, so retrying
// the same concrete owner resumes at the first unconsumed resource.
type reachFactConstructionFailure struct {
	owner             *ReachFact
	runtime           *memory.Runtime
	cause, cleanupErr error
	reported          error
	reportText        string
}

func (f *reachFactConstructionFailure) Error() string                 { return f.reportText }
func (f *reachFactConstructionFailure) Unwrap() error                 { return f.reported }
func (f *reachFactConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *reachFactConstructionFailure) OriginalCause() error          { return f.cause }
func (f *reachFactConstructionFailure) CleanupCause() error           { return f.cleanupErr }
func (f *reachFactConstructionFailure) Cleanup() error {
	if f.owner == nil {
		f.cleanupErr = nil
		return nil
	}
	f.cleanupErr = f.owner.Close()
	if f.cleanupErr == nil {
		f.owner = nil
	}
	return f.cleanupErr
}

var _ memory.ConstructionFailure = (*reachFactConstructionFailure)(nil)

func finishReachFactConstruction(cause error, owner *ReachFact) error {
	if owner == nil {
		return cause
	}
	runtime := owner.state.manager.NumeralService().RuntimeOwner()
	cleanup := owner.Close()
	if cleanup == nil {
		return cause
	}
	reported := errors.Join(cause, cleanup)
	return &reachFactConstructionFailure{owner: owner, runtime: runtime, cause: cause, cleanupErr: cleanup, reported: reported, reportText: reported.Error()}
}

type reachFactState struct {
	manager        *ast.ASTManager
	fact           ast.ASTRef
	auxiliaries    []ast.ASTRef
	rule           *base.Rule
	justifications []*ReachFact
	initial        bool
	references     uint32
	released       bool
}

// ReachFact is one owning reference to a retained Spacer reachability fact.
type ReachFact struct {
	state  *reachFactState
	closed bool
}

func NewReachFact(manager *ast.ASTManager, rule *base.Rule, fact ast.ASTHandle, auxiliaries []ast.ASTHandle, initial bool) (_ *ReachFact, resultErr error) {
	if manager == nil || rule == nil {
		return nil, ErrManagerOwner
	}
	ownedRule, err := rule.Retain()
	if err != nil {
		return nil, err
	}
	state := &reachFactState{manager: manager, rule: ownedRule, initial: initial, references: 1}
	result := &ReachFact{state: state}
	defer func() {
		if resultErr != nil {
			resultErr = finishReachFactConstruction(resultErr, result)
		}
	}()
	state.fact, err = manager.NewRef(fact)
	if err != nil {
		return nil, err
	}
	state.auxiliaries, err = refsFromHandles(manager, auxiliaries)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (f *ReachFact) open() (*reachFactState, error) {
	if f == nil || f.closed || f.state == nil || f.state.released || f.state.manager == nil {
		return nil, ErrManagerOwner
	}
	return f.state, nil
}

func (f *ReachFact) Retain() (*ReachFact, error) {
	state, err := f.open()
	if err != nil || state.references == ^uint32(0) {
		if err == nil {
			err = ErrManagerOwner
		}
		return nil, err
	}
	state.references++
	return &ReachFact{state: state}, nil
}

func (f *ReachFact) Fact() (ast.ASTHandle, error) {
	state, err := f.open()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.RefGet(state.fact)
}

func (f *ReachFact) Rule() (*base.Rule, error) {
	state, err := f.open()
	if err != nil {
		return nil, err
	}
	return state.rule.Retain()
}

func (f *ReachFact) IsInitial() (bool, error) {
	state, err := f.open()
	if err != nil {
		return false, err
	}
	return state.initial, nil
}

func (f *ReachFact) Auxiliaries() ([]ast.ASTHandle, error) {
	state, err := f.open()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(state.auxiliaries))
	for i := range state.auxiliaries {
		result[i], err = state.manager.RefGet(state.auxiliaries[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (f *ReachFact) AddJustification(justification *ReachFact) error {
	state, err := f.open()
	if err != nil {
		return err
	}
	if justification == nil {
		return ErrManagerOwner
	}
	other, err := justification.open()
	if err != nil || other.manager != state.manager {
		if err == nil {
			err = ErrManagerOwner
		}
		return err
	}
	owned, err := justification.Retain()
	if err != nil {
		return err
	}
	state.justifications = append(state.justifications, owned)
	return nil
}

func (f *ReachFact) Justifications() ([]*ReachFact, error) {
	state, err := f.open()
	if err != nil {
		return nil, err
	}
	result := make([]*ReachFact, 0, len(state.justifications))
	for _, justification := range state.justifications {
		owned, err := justification.Retain()
		if err != nil {
			for _, prior := range result {
				_ = prior.Close()
			}
			return nil, err
		}
		result = append(result, owned)
	}
	return result, nil
}

// Close releases one owner. Final destruction is refused before changing
// ownership while the AST manager is executing, and unconsumed roots remain
// attached if a later release fails.
func (f *ReachFact) Close() error {
	if f == nil || f.closed {
		return nil
	}
	state, err := f.open()
	if err != nil {
		return err
	}
	if state.references > 1 {
		state.references--
		f.closed = true
		f.state = nil
		return nil
	}
	if _, err = state.manager.ProofMode(); err != nil {
		return err
	}
	for len(state.justifications) > 0 {
		last := len(state.justifications) - 1
		if err = state.justifications[last].Close(); err != nil {
			return err
		}
		state.justifications[last] = nil
		state.justifications = state.justifications[:last]
	}
	for len(state.auxiliaries) > 0 {
		last := len(state.auxiliaries) - 1
		err = state.manager.DelRef(&state.auxiliaries[last])
		if state.auxiliaries[last] == (ast.ASTRef{}) {
			state.auxiliaries = state.auxiliaries[:last]
		}
		if err != nil {
			return err
		}
	}
	if state.fact != (ast.ASTRef{}) {
		if err = state.manager.DelRef(&state.fact); err != nil {
			return err
		}
	}
	if state.rule != nil {
		if err = state.rule.Close(); err != nil {
			return err
		}
		state.rule = nil
	}
	state.references = 0
	state.manager = nil
	state.released = true
	f.closed = true
	f.state = nil
	return nil
}
