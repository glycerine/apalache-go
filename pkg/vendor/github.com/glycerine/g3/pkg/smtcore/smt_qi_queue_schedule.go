package smtcore

// Source: src/smt/qi_queue.h and src/smt/qi_queue.cpp queue scheduling,
// scope rollback, delayed-instance summaries, and statistics.

import "github.com/glycerine/g3/pkg/ast"

type SMTQIQueueEntry struct {
	Fingerprint  *SMTFingerprint
	Cost         float32
	Generation   uint32
	Instantiated bool
}

type SMTQIQueueStats struct {
	Instances         uint32
	LazyInstances     uint32
	MissedInstances   uint32
	MinimumMissedCost float32
	MaximumMissedCost float32
}

type SMTQIDelayedSummary struct {
	Quantifier  ast.ASTHandle
	Count       uint32
	MinimumCost float32
	MaximumCost float32
}

type smtQIQueueScope struct {
	delayedLimit      int
	instanceLimit     int
	instantiatedLimit int
}

type SMTQIQueueSchedule struct {
	manager           *ast.ASTManager
	eagerThreshold    float32
	lazyThreshold     float32
	conservativeFinal bool
	newEntries        []SMTQIQueueEntry
	delayedEntries    []SMTQIQueueEntry
	instances         []ast.ASTHandle
	instanceRefs      []ast.ASTRef
	instantiatedTrail []int
	scopes            []smtQIQueueScope
	stats             SMTQIQueueStats
}

func NewSMTQIQueueSchedule(manager *ast.ASTManager, eagerThreshold, lazyThreshold float32, conservativeFinal bool) *SMTQIQueueSchedule {
	return &SMTQIQueueSchedule{manager: manager, eagerThreshold: eagerThreshold, lazyThreshold: lazyThreshold, conservativeFinal: conservativeFinal}
}

func (q *SMTQIQueueSchedule) Insert(fingerprint *SMTFingerprint, cost float32, generation uint32) error {
	if q == nil {
		return ast.ErrASTHandle
	}
	if fingerprint == nil || fingerprint.data.IsNull() {
		return ast.ErrASTDomain
	}
	q.newEntries = append(q.newEntries, SMTQIQueueEntry{Fingerprint: fingerprint, Cost: cost, Generation: generation})
	return nil
}

func (q *SMTQIQueueSchedule) HasWork() bool {
	return q != nil && len(q.newEntries) != 0
}

// ScheduleNew applies the eager threshold. promoted identifies entries whose
// instantiated formula is already known to conflict, matching qi_promote_unsat.
func (q *SMTQIQueueSchedule) ScheduleNew(promoted map[*SMTFingerprint]bool) ([]SMTQIQueueEntry, error) {
	if q == nil {
		return nil, ast.ErrASTHandle
	}
	ready := make([]SMTQIQueueEntry, 0, len(q.newEntries))
	for i := range q.newEntries {
		entry := q.newEntries[i]
		if entry.Cost <= q.eagerThreshold || promoted[entry.Fingerprint] {
			entry.Instantiated = true
			ready = append(ready, entry)
		} else {
			q.delayedEntries = append(q.delayedEntries, entry)
		}
	}
	q.newEntries = nil
	return ready, nil
}

// FinalCheck selects delayed entries exactly as qi_queue::final_check_eh. The
// Boolean result is true when no entry was selected and final checking is done.
func (q *SMTQIQueueSchedule) FinalCheck() (bool, []SMTQIQueueEntry, error) {
	if q == nil {
		return false, nil, ast.ErrASTHandle
	}
	limit := q.lazyThreshold
	if q.conservativeFinal {
		found := false
		for i := range q.delayedEntries {
			entry := q.delayedEntries[i]
			if !entry.Instantiated && entry.Cost <= q.lazyThreshold && (!found || entry.Cost < limit) {
				found = true
				limit = entry.Cost
			}
		}
		if !found {
			return true, nil, nil
		}
	}
	var ready []SMTQIQueueEntry
	for i := range q.delayedEntries {
		entry := &q.delayedEntries[i]
		if entry.Instantiated || entry.Cost > limit {
			continue
		}
		entry.Instantiated = true
		q.instantiatedTrail = append(q.instantiatedTrail, i)
		q.stats.LazyInstances++
		ready = append(ready, *entry)
	}
	return len(ready) == 0, ready, nil
}

func (q *SMTQIQueueSchedule) RecordInstance(lemma ast.ASTHandle) error {
	if q == nil || q.manager == nil {
		return ast.ErrASTHandle
	}
	if lemma.IsNull() {
		return ast.ErrASTDomain
	}
	ref, err := q.manager.NewRef(lemma)
	if err != nil {
		return err
	}
	q.instances = append(q.instances, lemma)
	q.instanceRefs = append(q.instanceRefs, ref)
	q.stats.Instances++
	return nil
}

func (q *SMTQIQueueSchedule) PushScope() error {
	if q == nil {
		return ast.ErrASTHandle
	}
	if len(q.newEntries) != 0 {
		return ast.ErrASTDomain
	}
	q.scopes = append(q.scopes, smtQIQueueScope{
		delayedLimit: len(q.delayedEntries), instanceLimit: len(q.instances), instantiatedLimit: len(q.instantiatedTrail),
	})
	return nil
}

func (q *SMTQIQueueSchedule) PopScopes(count uint32) error {
	if q == nil {
		return ast.ErrASTHandle
	}
	if count == 0 || count > uint32(len(q.scopes)) {
		return ast.ErrASTDomain
	}
	newLevel := len(q.scopes) - int(count)
	scope := q.scopes[newLevel]
	if err := q.releaseInstances(scope.instanceLimit); err != nil {
		return err
	}
	for _, index := range q.instantiatedTrail[scope.instantiatedLimit:] {
		if index < 0 || index >= len(q.delayedEntries) {
			return ast.ErrASTDomain
		}
		q.delayedEntries[index].Instantiated = false
	}
	q.instantiatedTrail = q.instantiatedTrail[:scope.instantiatedLimit]
	q.delayedEntries = q.delayedEntries[:scope.delayedLimit]
	q.newEntries = nil
	q.scopes = q.scopes[:newLevel]
	return nil
}

func (q *SMTQIQueueSchedule) releaseInstances(limit int) error {
	for len(q.instanceRefs) > limit {
		index := len(q.instanceRefs) - 1
		if err := q.manager.DelRef(&q.instanceRefs[index]); err != nil {
			return err
		}
		q.instanceRefs = q.instanceRefs[:index]
		q.instances = q.instances[:index]
	}
	return nil
}

func (q *SMTQIQueueSchedule) Reset() error {
	if q == nil {
		return nil
	}
	if err := q.releaseInstances(0); err != nil {
		return err
	}
	q.newEntries = nil
	q.delayedEntries = nil
	q.instances = nil
	q.instantiatedTrail = nil
	q.scopes = nil
	return nil
}

func (q *SMTQIQueueSchedule) DelayedSummaries() []SMTQIDelayedSummary {
	if q == nil {
		return nil
	}
	index := make(map[ast.ASTHandle]int)
	var summaries []SMTQIDelayedSummary
	for _, entry := range q.delayedEntries {
		if entry.Instantiated {
			continue
		}
		quantifier := entry.Fingerprint.data
		position, found := index[quantifier]
		if !found {
			index[quantifier] = len(summaries)
			summaries = append(summaries, SMTQIDelayedSummary{
				Quantifier: quantifier, Count: 1, MinimumCost: entry.Cost, MaximumCost: entry.Cost,
			})
			continue
		}
		summary := &summaries[position]
		summary.Count++
		if entry.Cost < summary.MinimumCost {
			summary.MinimumCost = entry.Cost
		}
		// Preserve the source's observed max-cost update, which takes min.
		if entry.Cost < summary.MaximumCost {
			summary.MaximumCost = entry.Cost
		}
	}
	return summaries
}

func (q *SMTQIQueueSchedule) Stats() SMTQIQueueStats {
	if q == nil {
		return SMTQIQueueStats{}
	}
	stats := q.stats
	stats.MissedInstances = uint32(len(q.delayedEntries))
	found := false
	for _, entry := range q.delayedEntries {
		if entry.Instantiated {
			continue
		}
		if !found {
			found = true
			stats.MinimumMissedCost, stats.MaximumMissedCost = entry.Cost, entry.Cost
			continue
		}
		if entry.Cost < stats.MinimumMissedCost {
			stats.MinimumMissedCost = entry.Cost
		}
		if entry.Cost > stats.MaximumMissedCost {
			stats.MaximumMissedCost = entry.Cost
		}
	}
	return stats
}
