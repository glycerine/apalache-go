package spacer

// Source: spacer_context.cpp expand_node's ordered lemma generalizer loop.
import "errors"

// generalizeBlockingLemma consumes lemma and returns the resulting owner.
func (p *PredicateTransformer) generalizeBlockingLemma(obligation *ProofObligation, lemma *Lemma) (*Lemma, error) {
	if p.useEqClassGeneralizer {
		generalize, err := shouldGeneralizeLemma(obligation, lemma)
		if err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		if !generalize {
			return lemma, nil
		}
		if err := spacerCheckpoint(p.manager); err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		generalized, generalizeErr := p.generalizeEqualityClasses(obligation, lemma)
		if generalizeErr != nil {
			return nil, errors.Join(generalizeErr, lemma.Close())
		}
		if generalized != lemma {
			if closeErr := lemma.Close(); closeErr != nil {
				return nil, errors.Join(closeErr, generalized.Close())
			}
			lemma = generalized
		}
	}
	if p.useInductiveGeneralizer {
		generalize, err := shouldGeneralizeLemma(obligation, lemma)
		if err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		if !generalize {
			return lemma, nil
		}
		if err := spacerCheckpoint(p.manager); err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		generalized, generalizeErr := p.generalizeInductiveLemma(obligation, lemma)
		if generalizeErr != nil {
			return nil, errors.Join(generalizeErr, lemma.Close())
		}
		if generalized != lemma {
			if closeErr := lemma.Close(); closeErr != nil {
				return nil, errors.Join(closeErr, generalized.Close())
			}
			lemma = generalized
		}
	}
	if p.useArrayEqGeneralizer {
		generalize, err := shouldGeneralizeLemma(obligation, lemma)
		if err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		if !generalize {
			return lemma, nil
		}
		if err := spacerCheckpoint(p.manager); err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		generalized, generalizeErr := p.generalizeArrayEqualityLemma(obligation, lemma)
		if generalizeErr != nil {
			return nil, errors.Join(generalizeErr, lemma.Close())
		}
		if generalized != lemma {
			if closeErr := lemma.Close(); closeErr != nil {
				return nil, errors.Join(closeErr, generalized.Close())
			}
			lemma = generalized
		}
	}
	if p.lemmaSanityCheck {
		generalize, err := shouldGeneralizeLemma(obligation, lemma)
		if err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		if !generalize {
			return lemma, nil
		}
		if err := spacerCheckpoint(p.manager); err != nil {
			return nil, errors.Join(err, lemma.Close())
		}
		if sanityErr := p.checkLemmaSanity(lemma); sanityErr != nil {
			return nil, errors.Join(sanityErr, lemma.Close())
		}
	}
	return lemma, nil
}
