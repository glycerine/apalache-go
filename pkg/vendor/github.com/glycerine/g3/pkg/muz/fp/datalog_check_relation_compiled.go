package fp

// Source: src/muz/rel/rel_context.cpp checked favourite-plugin query scope.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/muz/rel"
)

// datalogCompiledCheckScope owns the temporary manager exchange. The ordinary
// checker scope continues to own the pair objects after Close restores the
// original manager relations.
type datalogCompiledCheckScope struct {
	check     *datalogCheckRelationScope
	installed []int
	retired   []rel.FiniteTable
	factory   bool
}

func (e *PropositionalEngine) datalogDefaultCheckedPrimary() (primary string, resultErr error) {
	if e.datalogDefaultTable == "" {
		if err := e.applyDatalogDefaultFiniteTable(); err != nil {
			return "", err
		}
	}
	primary = e.datalogDefaultTable
	switch primary {
	case "tr_hashtable":
		primary = "hashtable"
	case "tr_sparse":
		primary = "sparse"
	case "tr_lazy_sparse":
		primary = "lazy_sparse"
	}
	if primary != "hashtable" && primary != "sparse" && primary != "lazy_sparse" {
		return "", fmt.Errorf("muz/fp: checked default table %s is not available", primary)
	}
	return primary, nil
}

// beginCompiled installs checked predicate owners into the relation manager
// and selects checked construction for compiler temporaries. Formula-backed
// datalog.check_relation scopes remain interpreted.
func (s *datalogCheckRelationScope) beginCompiled() (_ *datalogCompiledCheckScope, supported bool, resultErr error) {
	if s == nil || s.engine == nil {
		return nil, false, nil
	}
	for i := range s.entries {
		if s.entries[i].pair == nil || s.entries[i].nested != nil || s.entries[i].checked != nil {
			return nil, false, nil
		}
	}
	if s.factory == nil {
		return nil, false, nil
	}
	compiled := &datalogCompiledCheckScope{check: s}
	s.compiled = compiled
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, compiled.Close())
		}
	}()
	if err := s.engine.relations.SetCheckedFiniteFactory(s.factory); err != nil {
		return nil, false, err
	}
	compiled.factory = true
	for i := range s.entries {
		entry := &s.entries[i]
		original, err := s.engine.relations.ExchangeFiniteRelation(entry.declaration, entry.pair)
		if err != nil {
			return nil, false, err
		}
		entry.original = original
		entry.state.table = entry.pair
		compiled.installed = append(compiled.installed, i)
	}
	return compiled, true, nil
}

// Close clears checked temporary construction, restores each original manager
// owner, and transfers the current compiled checked result back to the
// ordinary checker scope for publication and disposal. It is retryable.
func (s *datalogCompiledCheckScope) Close() (result error) {
	if s == nil || s.check == nil || s.check.engine == nil {
		return nil
	}
	if s.factory {
		if err := s.check.engine.relations.SetCheckedFiniteFactory(nil); err != nil {
			result = errors.Join(result, err)
		} else {
			s.factory = false
		}
	}
	retired := s.retired[:0]
	for _, table := range s.retired {
		if err := table.Close(); err != nil {
			result = errors.Join(result, err)
			retired = append(retired, table)
		}
	}
	s.retired = retired
	remaining := s.installed[:0]
	for _, index := range s.installed {
		entry := &s.check.entries[index]
		current, err := s.check.engine.relations.ExchangeFiniteRelation(entry.declaration, entry.original)
		if err != nil {
			result = errors.Join(result, err)
			remaining = append(remaining, index)
			continue
		}
		pair, ok := current.(*rel.FiniteCheckedPair)
		if !ok {
			// The original is already restored in the manager. Retire the
			// unexpected displaced owner here; if cleanup refuses, retain it for
			// the next Close retry instead of attempting an ambiguous rollback.
			closeErr := current.Close()
			if closeErr != nil {
				s.retired = append(s.retired, current)
			}
			entry.state.table = entry.original
			result = errors.Join(result, rel.ErrCheckTable, closeErr)
			continue
		}
		entry.pair = pair
		// Keep query status on the compiled result until ordinary scope
		// completion publishes it and restores persistent state.
		entry.state.table = pair
	}
	s.installed = remaining
	if len(s.installed) == 0 && len(s.retired) == 0 && !s.factory {
		s.check.compiled = nil
		s.check = nil
	}
	return result
}
