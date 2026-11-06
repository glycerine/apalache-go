package fp

// Temporary query contexts remain owned until their source-local destruction
// finishes. A refused cleanup must not publish a result or discard its roots.

import (
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/muz/spacer"
)

type queryCloneCleanup struct {
	clone           *PropositionalEngine
	pdr             pdr.StatisticsSnapshot
	spacer          spacer.StatisticsSnapshot
	statisticsReady bool
}

func (c *queryCloneCleanup) captureStatistics() error {
	if c.statisticsReady {
		return nil
	}
	var err error
	switch c.clone.backend {
	case "pdr", "qpdr":
		c.pdr, err = c.clone.snapshotPDRStatistics()
	case "spacer":
		c.spacer, err = c.clone.snapshotSpacerStatistics()
	}
	if err == nil {
		c.statisticsReady = true
	}
	return err
}

func (e *PropositionalEngine) discardQueryClone(clone *PropositionalEngine) error {
	e.queryCleanup = append(e.queryCleanup, &queryCloneCleanup{clone: clone})
	return e.clearQueryCleanup()
}

func (e *PropositionalEngine) clearQueryCleanup() error {
	for len(e.queryCleanup) != 0 {
		pending := e.queryCleanup[0]
		if err := pending.captureStatistics(); err != nil {
			return err
		}
		if err := pending.clone.Close(); err != nil {
			return err
		}
		switch pending.clone.backend {
		case "pdr", "qpdr":
			e.pdrStatistics.Merge(pending.pdr)
		case "spacer":
			e.spacerStatistics.Merge(pending.spacer)
		}
		e.queryCleanup[0] = nil
		e.queryCleanup = e.queryCleanup[1:]
	}
	e.queryCleanup = nil
	return nil
}
