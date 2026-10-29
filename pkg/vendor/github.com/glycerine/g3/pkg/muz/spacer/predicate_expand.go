package spacer

// Source: src/muz/spacer/spacer_context.cpp context::expand_node reachable,
// abstract-child, and blocking-lemma branches.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrWeakAbstractionExhausted = errors.New("muz/spacer: weak abstraction exhausted")

type ExpansionResult struct {
	Status     lbool.LBool
	Child      *ProofObligation
	Concrete   bool
	LemmaAdded bool
}

func shouldGeneralizeLemma(obligation *ProofObligation, lemma *Lemma) (bool, error) {
	useFarkas, err := obligation.UseFarkas()
	if err != nil || !useFarkas {
		return false, err
	}
	body, err := lemma.Body()
	if err != nil {
		return false, err
	}
	return !lemma.manager.IsFalse(body), nil
}

// ExpandObligation applies one Spacer search expansion without queue policy or
// lemma generalizers. Undef with a child means search continues at that child.
func (p *PredicateTransformer) ExpandObligation(obligation *ProofObligation) (_ ExpansionResult, resultErr error) {
	if err := p.open(); err != nil {
		return ExpansionResult{}, err
	}
	check, err := p.CheckReachability(obligation)
	if err != nil {
		return ExpansionResult{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, check.Close(p.manager)) }()
	result := ExpansionResult{Status: check.Status, Concrete: check.IsConcrete}
	switch check.Status {
	case lbool.True:
		if check.Rule == nil {
			if err = obligation.Close(); err != nil {
				return ExpansionResult{}, err
			}
			return result, nil
		}
		predecessors, err := p.RulePredecessors(check.Rule)
		if err != nil {
			return ExpansionResult{}, err
		}
		if check.IsConcrete {
			if len(predecessors) != 0 {
				fact, makeErr := p.makeConfiguredReachFact(obligation, check.Rule, check.Model)
				if makeErr != nil {
					return ExpansionResult{}, makeErr
				}
				_, addErr := p.AddReachFact(fact)
				closeErr := fact.Close()
				if addErr != nil || closeErr != nil {
					return ExpansionResult{}, errors.Join(addErr, closeErr)
				}
			}
			derivation, found, detachErr := obligation.Derivation()
			if detachErr != nil {
				return ExpansionResult{}, detachErr
			}
			if found {
				derivation, detachErr = obligation.DetachDerivation()
				if detachErr != nil {
					return ExpansionResult{}, detachErr
				}
			}
			if err = obligation.Close(); err != nil {
				if derivation != nil {
					_ = derivation.Close()
				}
				return ExpansionResult{}, err
			}
			if derivation != nil {
				child, childErr := derivation.CreateNextChild()
				if childErr != nil {
					return ExpansionResult{}, errors.Join(childErr, derivation.Close())
				}
				if child != nil {
					if childErr = child.SetDerivation(derivation); childErr != nil {
						return ExpansionResult{}, errors.Join(childErr, derivation.Close())
					}
					result.Status = lbool.Undef
					result.Child = child
					return result, nil
				}
				if err = derivation.Close(); err != nil {
					return ExpansionResult{}, err
				}
			}
			return result, nil
		}
		child, err := p.CreateDerivationChild(obligation, check.Rule, check.Model, check.ReachPredUsed, p.reverseChildren)
		if err != nil {
			return ExpansionResult{}, err
		}
		if child == nil {
			return ExpansionResult{}, ast.ErrASTDomain
		}
		result.Status = lbool.Undef
		result.Child = child
		return result, nil
	case lbool.False:
		cube := make([]ast.ASTHandle, len(check.Core))
		for i := range check.Core {
			cube[i], err = p.manager.RefGet(check.Core[i])
			if err != nil {
				return ExpansionResult{}, err
			}
		}
		lemma, err := NewLemmaFromProofObligation(obligation, cube, check.UsesLevel)
		if err != nil {
			return ExpansionResult{}, err
		}
		generalize, err := shouldGeneralizeLemma(obligation, lemma)
		if err != nil {
			return ExpansionResult{}, errors.Join(err, lemma.Close())
		}
		if generalize && p.useEqClassGeneralizer {
			generalized, generalizeErr := p.generalizeEqualityClasses(obligation, lemma)
			if generalizeErr != nil {
				return ExpansionResult{}, errors.Join(generalizeErr, lemma.Close())
			}
			if generalized != lemma {
				if closeErr := lemma.Close(); closeErr != nil {
					return ExpansionResult{}, errors.Join(closeErr, generalized.Close())
				}
				lemma = generalized
			}
		}
		if generalize && p.useInductiveGeneralizer {
			generalized, generalizeErr := p.generalizeInductiveLemma(obligation, lemma)
			if generalizeErr != nil {
				return ExpansionResult{}, errors.Join(generalizeErr, lemma.Close())
			}
			if generalized != lemma {
				if closeErr := lemma.Close(); closeErr != nil {
					return ExpansionResult{}, errors.Join(closeErr, generalized.Close())
				}
				lemma = generalized
			}
		}
		if generalize && p.useArrayEqGeneralizer {
			generalized, generalizeErr := p.generalizeArrayEqualityLemma(obligation, lemma)
			if generalizeErr != nil {
				return ExpansionResult{}, errors.Join(generalizeErr, lemma.Close())
			}
			if generalized != lemma {
				if closeErr := lemma.Close(); closeErr != nil {
					return ExpansionResult{}, errors.Join(closeErr, generalized.Close())
				}
				lemma = generalized
			}
		}
		if generalize && p.lemmaSanityCheck {
			if sanityErr := p.checkLemmaSanity(lemma); sanityErr != nil {
				return ExpansionResult{}, errors.Join(sanityErr, lemma.Close())
			}
		}
		var cti ast.ASTRef
		if p.useLemmaAsCTI {
			cube, cubeErr := lemma.Cube()
			if cubeErr != nil {
				return ExpansionResult{}, errors.Join(cubeErr, lemma.Close())
			}
			conjunction, conjunctionErr := p.manager.MkAndArray(cube)
			if conjunctionErr != nil {
				return ExpansionResult{}, errors.Join(conjunctionErr, lemma.Close())
			}
			cti, err = p.manager.NewRef(conjunction)
			if err != nil {
				return ExpansionResult{}, errors.Join(err, lemma.Close())
			}
			defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&cti)) }()
		}
		added, err := p.frames.AddLemma(lemma)
		if err != nil {
			return ExpansionResult{}, errors.Join(err, lemma.Close())
		}
		result.LemmaAdded = added
		if added && cti != (ast.ASTRef{}) {
			post, getErr := p.manager.RefGet(cti)
			if getErr != nil {
				return ExpansionResult{}, getErr
			}
			if getErr = obligation.NewPost(post); getErr != nil {
				return ExpansionResult{}, getErr
			}
			if getErr = obligation.SetUseFarkas(false); getErr != nil {
				return ExpansionResult{}, getErr
			}
		}
		return result, nil
	default:
		weakness, weaknessErr := obligation.Weakness()
		if weaknessErr != nil {
			return ExpansionResult{}, weaknessErr
		}
		if weakness >= 100 {
			return ExpansionResult{}, ErrWeakAbstractionExhausted
		}
		if check.Rule != nil && check.Model != nil {
			reachPredUsed := make([]bool, len(check.ReachPredUsed))
			child, childErr := p.CreateDerivationChild(obligation, check.Rule, check.Model, reachPredUsed, p.reverseChildren)
			if childErr != nil {
				return ExpansionResult{}, childErr
			}
			if child != nil {
				result.Child = child
				return result, nil
			}
		}
		if err = obligation.BumpWeakness(); err != nil {
			return ExpansionResult{}, err
		}
		if err = check.Close(p.manager); err != nil {
			return ExpansionResult{}, err
		}
		return p.ExpandObligation(obligation)
	}
}
