package spacer

// Source: src/muz/spacer/spacer_context.cpp context::check_reachability and
// context::is_reachable, using the source-default non-flexible trace policy.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/luby"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

type searchRestartPolicy struct {
	enabled          bool
	initialThreshold uint32
	threshold        uint32
	initialLemmas    uint64
	lubyIndex        uint32
}

func newSearchRestartPolicy(enabled bool, initialThreshold uint32, initialLemmas uint64) searchRestartPolicy {
	return searchRestartPolicy{enabled: enabled, initialThreshold: initialThreshold, threshold: initialThreshold, initialLemmas: initialLemmas, lubyIndex: 1}
}

func (p *searchRestartPolicy) observe(lemmas uint64) (bool, error) {
	if !p.enabled || lemmas-p.initialLemmas <= uint64(p.threshold) {
		return false, nil
	}
	p.lubyIndex++
	value, err := luby.Value(p.lubyIndex)
	if err != nil {
		return false, err
	}
	p.threshold = value * p.initialThreshold
	p.initialLemmas = lemmas
	return true, nil
}

func (q *ProofObligationQueue) confirmConcreteReachability(obligation *ProofObligation) (_ bool, resultErr error) {
	if q == nil || obligation == nil || obligation.valid() != nil {
		return false, ErrManagerOwner
	}
	if timers := obligation.transformer.contextTimers; timers != nil {
		watch, err := stopwatch.NewScopedWatch(&timers.isReach, false)
		if err != nil {
			return false, err
		}
		defer func() { resultErr = errors.Join(resultErr, watch.Close()) }()
	}
	saved := obligation.level
	obligation.level = InfinityLevel
	check, err := obligation.transformer.CheckReachability(obligation)
	obligation.level = saved
	if err != nil {
		return false, fmt.Errorf("spacer: confirm concrete reachability: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, check.Close(obligation.transformer.manager)) }()
	if check.Status != lbool.True || !check.IsConcrete {
		return false, nil
	}
	if check.Rule != nil {
		predecessors, err := obligation.transformer.RulePredecessors(check.Rule)
		if err != nil {
			return false, fmt.Errorf("spacer: confirm reachability predecessors: %w", err)
		}
		if len(predecessors) != 0 {
			fact, err := obligation.transformer.makeConfiguredReachFact(obligation, check.Rule, check.Model)
			if err != nil {
				return false, fmt.Errorf("spacer: confirm reachability fact: %w", err)
			}
			_, addErr := obligation.transformer.AddReachFact(fact)
			closeErr := fact.Close()
			if addErr != nil || closeErr != nil {
				return false, fmt.Errorf("spacer: confirm reachability add fact: %w", errors.Join(addErr, closeErr))
			}
		}
	}
	derivation, found, err := obligation.Derivation()
	if err != nil {
		return false, fmt.Errorf("spacer: confirm reachability derivation: %w", err)
	}
	if found {
		derivation, err = obligation.DetachDerivation()
		if err != nil {
			return false, fmt.Errorf("spacer: confirm reachability detach derivation: %w", err)
		}
	}
	if err = obligation.Close(); err != nil {
		if derivation != nil {
			_ = derivation.Close()
		}
		return false, fmt.Errorf("spacer: confirm reachability close obligation: %w", err)
	}
	if derivation == nil {
		return true, nil
	}
	next, err := derivation.CreateNextChild()
	if err != nil {
		return false, fmt.Errorf("spacer: confirm reachability next child: %w", errors.Join(err, derivation.Close()))
	}
	if next == nil {
		return true, derivation.Close()
	}
	if err = next.SetDerivation(derivation); err != nil {
		return false, fmt.Errorf("spacer: confirm reachability set derivation: %w", errors.Join(err, derivation.Close()))
	}
	// Source is_reachable publishes the next child before recursing. If the
	// recursive confirmation fails, ordinary queue search must still see it.
	top, err := q.Top()
	if err != nil {
		return false, err
	}
	if top == obligation {
		if _, err = q.Pop(); err != nil {
			return false, err
		}
	}
	if err = q.Push(next); err != nil {
		return false, err
	}
	return q.confirmConcreteReachability(next)
}

// CheckReachability runs one source-default obligation-queue search at the
// queue's current maximum level. True means a concrete trace reached the root;
// false means the root was blocked at this level.
func (q *ProofObligationQueue) CheckReachability() (bool, error) {
	return q.checkReachability(true, false, InfinityLevel, false, 0, nil)
}

func (q *ProofObligationQueue) checkReachability(reset, flexible bool, flexibleDepth uint32, useRestarts bool, restartInitial uint32, lemmaCount func() (uint64, error)) (_ bool, resultErr error) {
	if q == nil || q.root == nil || q.root.valid() != nil {
		return false, ErrManagerOwner
	}
	if timers := q.root.transformer.contextTimers; timers != nil {
		watch, err := stopwatch.NewScopedWatch(&timers.reach, false)
		if err != nil {
			return false, err
		}
		defer func() { resultErr = errors.Join(resultErr, watch.Close()) }()
	}
	if reset {
		if err := q.Reset(); err != nil {
			return false, err
		}
	}
	q.expanded = InfinityLevel
	var initialLemmas uint64
	if useRestarts {
		if lemmaCount == nil {
			return false, ErrManagerOwner
		}
		var err error
		initialLemmas, err = lemmaCount()
		if err != nil {
			return false, err
		}
	}
	restart := newSearchRestartPolicy(useRestarts, restartInitial, initialLemmas)
	needsCheckpoint := true
	for {
		node, err := q.Top()
		if err != nil {
			return false, err
		}
		if node == nil {
			return false, nil
		}
		if needsCheckpoint {
			if err := spacerCheckpoint(node.transformer.manager); err != nil {
				return false, err
			}
			needsCheckpoint = false
		}
		closed, err := node.IsClosed()
		if err != nil {
			return false, err
		}
		dirty, err := node.Dirty()
		if err != nil {
			return false, err
		}
		if closed || dirty {
			if _, err = q.Pop(); err != nil {
				return false, err
			}
			if closed {
				if q.IsRoot(node) {
					return true, nil
				}
			} else {
				if err = node.Clean(); err != nil {
					return false, err
				}
				if err = q.Push(node); err != nil {
					return false, err
				}
			}
			continue
		}
		if useRestarts {
			lemmas, countErr := lemmaCount()
			if countErr != nil {
				return false, countErr
			}
			restartNow, restartErr := restart.observe(lemmas)
			if restartErr != nil {
				return false, restartErr
			}
			if restartNow {
				q.restarts++
				if stats := q.root.transformer.contextStats; stats != nil {
					stats.restarts++
				}
				// Source pops only up to the root. Obligations ordered after
				// it remain pending for subsequent levels/depths.
				for {
					node, err = q.Top()
					if err != nil {
						return false, err
					}
					if node == nil {
						return false, ErrManagerOwner
					}
					if q.IsRoot(node) {
						break
					}
					if _, err = q.Pop(); err != nil {
						return false, err
					}
				}
			}
		}
		// context::expand_node returns immediately for a frame-blocked
		// flexible obligation, without generating a lemma or lowering expanded.
		if err = node.transformer.traceExtendedNode(node, q.MinimumDepth()); err != nil {
			return false, err
		}
		frameBlocked := false
		if flexible {
			frameBlocked, _, err = node.transformer.IsBlocked(node)
			if err != nil {
				return false, err
			}
		}
		expansion := ExpansionResult{Status: lbool.False}
		if !frameBlocked {
			expansion, err = node.transformer.ExpandObligation(node)
			if err != nil {
				return false, err
			}
		}
		switch expansion.Status {
		case lbool.True:
			if _, err = q.Pop(); err != nil {
				return false, err
			}
			if q.IsRoot(node) {
				return true, nil
			}
			// Source starts the next queue iteration before walking last_reachable.
			if err := spacerCheckpoint(node.transformer.manager); err != nil {
				return false, err
			}
			candidate := node
			for candidate.parent != nil {
				if err := spacerCheckpoint(node.transformer.manager); err != nil {
					return false, err
				}
				parent := candidate.parent
				reachable, err := q.confirmConcreteReachability(parent)
				if err != nil {
					return false, err
				}
				if !reachable {
					if closed, closeErr := parent.IsClosed(); closeErr != nil {
						return false, closeErr
					} else if !closed {
						if err = parent.BumpWeakness(); err != nil {
							return false, err
						}
					}
					break
				}
				candidate = parent
				if q.IsRoot(candidate) {
					if err := spacerCheckpoint(node.transformer.manager); err != nil {
						return false, err
					}
					return true, nil
				}
			}
			continue // finish this source iteration after the ancestor walk
		case lbool.False:
			level, levelErr := node.Level()
			if levelErr != nil {
				return false, levelErr
			}
			if !frameBlocked && level < q.expanded {
				q.expanded = level
			}
			if _, err = q.Pop(); err != nil {
				return false, err
			}
			if dirty, err = node.Dirty(); err != nil {
				return false, err
			} else if dirty {
				if err = node.Clean(); err != nil {
					return false, err
				}
			}
			if err = node.IncrementLevel(); err != nil {
				return false, err
			}
			if flexible {
				nodeLevel, levelErr := node.Level()
				if levelErr != nil {
					return false, levelErr
				}
				if nodeLevel >= q.maximum || q.maximum-nodeLevel <= flexibleDepth {
					if err = q.Push(node); err != nil {
						return false, err
					}
				}
			}
			if q.IsRoot(node) {
				return false, nil
			}
		case lbool.Undef:
			if expansion.Child == nil {
				return false, nil
			}
			closed, err = node.IsClosed()
			if err != nil {
				return false, err
			}
			if closed {
				top, topErr := q.Top()
				if topErr != nil {
					return false, topErr
				}
				if top == node {
					if _, err = q.Pop(); err != nil {
						return false, err
					}
				}
			}
			if err = q.Push(expansion.Child); err != nil {
				return false, err
			}
		}
		needsCheckpoint = true
	}
}
