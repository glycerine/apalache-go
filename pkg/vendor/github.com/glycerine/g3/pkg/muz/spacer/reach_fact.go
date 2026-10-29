package spacer

// Source: src/muz/spacer/spacer_context.h reach_fact.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

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
			resultErr = errors.Join(resultErr, result.Close())
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

func (f *ReachFact) Close() (result error) {
	if f == nil || f.closed {
		return nil
	}
	state, err := f.open()
	if err != nil {
		return err
	}
	f.closed = true
	f.state = nil
	state.references--
	if state.references != 0 {
		return nil
	}
	for i := len(state.justifications) - 1; i >= 0; i-- {
		result = errors.Join(result, state.justifications[i].Close())
	}
	result = errors.Join(result, releaseNormalizeRefs(state.manager, state.auxiliaries))
	if state.fact != (ast.ASTRef{}) {
		result = errors.Join(result, state.manager.DelRef(&state.fact))
	}
	result = errors.Join(result, state.rule.Close())
	if result == nil {
		state.justifications = nil
		state.auxiliaries = nil
		state.rule = nil
		state.manager = nil
		state.released = true
	}
	return result
}
