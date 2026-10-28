package apalache

import (
	"fmt"
	"regexp"
)

// FilteredTransitionExecutor is the Go counterpart of Scala's executor
// decorator. Embedding preserves the complete executor API while the two
// filtered operations apply whole-word regular-expression matching.
type FilteredTransitionExecutor struct {
	*TransitionExecutor
	transitionText string
	invariantText  string
	transition     *regexp.Regexp
	invariant      *regexp.Regexp
}

func NewFilteredTransitionExecutor(
	transitionFilter, invariantFilter string, executor *TransitionExecutor,
) (*FilteredTransitionExecutor, error) {
	if executor == nil {
		return nil, fmt.Errorf("filtered transition executor requires an executor")
	}
	compile := func(name, pattern string) (*regexp.Regexp, error) {
		if pattern == "" {
			return nil, nil
		}
		compiled, err := regexp.Compile("^(?:" + pattern + ")$")
		if err != nil {
			return nil, fmt.Errorf("invalid %s filter %q: %w", name, pattern, err)
		}
		return compiled, nil
	}
	transition, err := compile("transition", transitionFilter)
	if err != nil {
		return nil, err
	}
	invariant, err := compile("invariant", invariantFilter)
	if err != nil {
		return nil, err
	}
	return &FilteredTransitionExecutor{
		TransitionExecutor: executor, transitionText: transitionFilter, invariantText: invariantFilter,
		transition: transition, invariant: invariant,
	}, nil
}

func (executor *FilteredTransitionExecutor) PrepareTransition(number int, expression Expr) (bool, error) {
	word := fmt.Sprintf("%d->%d", executor.Step(), number)
	if executor.transition != nil && !executor.transition.MatchString(word) {
		return false, nil
	}
	return executor.TransitionExecutor.PrepareTransition(number, expression)
}

func (executor *FilteredTransitionExecutor) MayChangeAssertion(
	transitionNumber int, kind InvariantKind, invariantNumber int, assertion Expr,
) (bool, error) {
	thisWord := fmt.Sprintf("%d->%s%d", executor.Step(), kind, invariantNumber)
	previousWord := fmt.Sprintf("%d->%s%d", executor.Step()-1, kind, invariantNumber)
	previousExcluded := executor.Step() > 0 && executor.invariant != nil && !executor.invariant.MatchString(previousWord)
	thisExcluded := executor.invariant != nil && !executor.invariant.MatchString(thisWord)
	mayChange, err := executor.TransitionExecutor.MayChangeAssertion(
		transitionNumber, kind, invariantNumber, assertion)
	if err != nil {
		return false, err
	}
	return !thisExcluded && (previousExcluded || mayChange), nil
}
