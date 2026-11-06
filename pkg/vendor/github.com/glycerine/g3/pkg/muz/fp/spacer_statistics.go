package fp

import (
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (e *PropositionalEngine) snapshotSpacerStatistics() (spacer.StatisticsSnapshot, error) {
	var result spacer.StatisticsSnapshot
	result.Merge(e.spacerStatistics)
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "spacer" {
			if err := pending.captureStatistics(); err != nil {
				return result, err
			}
			result.Merge(pending.spacer)
		}
	}
	if e.spacer != nil {
		if pending := e.spacerDisposal; pending != nil {
			result.Merge(pending.statistics)
		} else {
			current, err := e.spacer.SnapshotStatistics()
			if err != nil {
				return result, err
			}
			result.Merge(current)
		}
	}
	if e.queryClone != nil && e.queryClone.backend == "spacer" {
		if pending := e.queryCloneDisposal; pending != nil && pending.statisticsReady && pending.clone.closing {
			result.Merge(pending.spacer)
		} else {
			current, err := e.queryClone.snapshotSpacerStatistics()
			if err != nil {
				return result, err
			}
			result.Merge(current)
		}
	}
	return result, nil
}

func (e *PropositionalEngine) collectSpacerStatistics(destination *statistics.Statistics, previous spacer.StatisticsSnapshot) error {
	previous.Merge(e.spacerStatistics)
	if pending := e.spacerDisposal; pending != nil {
		previous.Merge(pending.statistics)
	}
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "spacer" {
			if err := pending.captureStatistics(); err != nil {
				return err
			}
			previous.Merge(pending.spacer)
		}
	}
	if e.queryClone != nil && e.queryClone.backend == "spacer" {
		if pending := e.queryCloneDisposal; pending != nil && pending.statisticsReady && pending.clone.closing {
			previous.Merge(pending.spacer)
		} else {
			if e.spacer != nil && e.spacerDisposal == nil {
				current, err := e.spacer.SnapshotStatistics()
				if err != nil {
					return err
				}
				previous.Merge(current)
			}
			return e.queryClone.collectSpacerStatistics(destination, previous)
		}
	}
	if e.spacer != nil && e.spacerDisposal == nil {
		return e.spacer.CollectStatisticsWithSnapshot(destination, previous)
	}
	return previous.Collect(destination)
}
