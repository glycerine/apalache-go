package fp

// Source: src/muz/rel/rel_context.cpp checked favourite table selection for
// explanation transform/compiler temporaries.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/muz/rel"
)

// datalogExplanationCheckedScope owns only the temporary factory installation.
// The relation manager owns every table created through the borrowed factory.
type datalogExplanationCheckedScope struct {
	engine                   *PropositionalEngine
	factory                  *rel.FiniteCheckedFactory
	primary                  string
	checker                  string
	uncheckedNullaryFallback bool
	installed                bool
}

func (e *PropositionalEngine) beginDatalogExplanationCheckedScope() (_ *datalogExplanationCheckedScope, enabled bool, resultErr error) {
	checker, enabled, err := e.datalogDefaultTableCheck()
	if err != nil || !enabled {
		return nil, false, err
	}
	primary, err := e.datalogDefaultCheckedPrimary()
	if err != nil {
		return nil, false, err
	}
	if (primary == "lazy_sparse" || checker == "lazy_sparse") && e.lazyFinite == nil {
		e.lazyFinite = rel.NewLazySparseFiniteFactory()
		if err = e.relations.SetLazySparseFiniteFactory(e.lazyFinite); err != nil {
			return nil, false, err
		}
	}
	factory, err := rel.NewFiniteCheckedFactoryWithLazy(primary, checker, e.lazyFinite)
	if err != nil {
		return nil, false, err
	}
	scope := &datalogExplanationCheckedScope{
		engine:                   e,
		factory:                  factory,
		primary:                  primary,
		checker:                  checker,
		uncheckedNullaryFallback: factory.AllowsUncheckedNullaryFallback(),
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, scope.Close())
		}
	}()
	if err = e.relations.SetCheckedFiniteFactory(factory); err != nil {
		return nil, false, err
	}
	scope.installed = true
	return scope, true, nil
}

type datalogExplanationCheckedProduct interface {
	CheckedConcreteKinds() (primary, checker string, checked bool, err error)
}

type datalogExplanationFiniteSignature interface {
	FiniteSignature() (rel.Signature, error)
}

// requireProduct proves the explanation product's finite/index component was
// actually constructed by this scope before a checked query may succeed.
func (s *datalogExplanationCheckedScope) requireProduct(product datalogExplanationCheckedProduct) error {
	if s == nil || !s.installed {
		return nil
	}
	primary, checker, checked, err := product.CheckedConcreteKinds()
	if err != nil {
		return err
	}
	if !checked {
		// A per-fact explanation table has one functional index column. Hash
		// cannot represent it, so source plugin selection falls through to an
		// ordinary Sparse index when either side of the checked favourite is
		// Hash. Accept only that concrete product and factory-proved boundary.
		if factProduct, fact := product.(*rel.FiniteExplanationProduct); fact && primary == "sparse" && checker == "" && s.factory != nil {
			signature, signatureErr := factProduct.IndexSignature()
			if signatureErr != nil {
				return signatureErr
			}
			if s.factory.AllowsUncheckedExplanationIndexFallback(signature, 1) {
				return nil
			}
		}
		// sparse/lazy plugins cannot represent the empty signature. Source
		// favourite selection deliberately falls back to ordinary Hash here;
		// accept only that proved capability boundary, never a generic bypass.
		signatureProduct, hasSignature := product.(datalogExplanationFiniteSignature)
		if s.uncheckedNullaryFallback && hasSignature && primary == "hashtable" {
			signature, signatureErr := signatureProduct.FiniteSignature()
			if signatureErr != nil {
				return signatureErr
			}
			if signature.Len() == 0 {
				return nil
			}
		}
		return fmt.Errorf("%w: explanation product kinds %s/%s checked=false", rel.ErrCheckTable, primary, checker)
	}
	return nil
}

// Close is retryable and restores ordinary favourite-table construction.
func (s *datalogExplanationCheckedScope) Close() error {
	if s == nil || !s.installed {
		return nil
	}
	if s.engine == nil || s.engine.relations == nil {
		return rel.ErrTable
	}
	if err := s.engine.relations.SetCheckedFiniteFactory(nil); err != nil {
		return err
	}
	s.installed = false
	return nil
}
