package fp

// Source: the PDR dl_interface check_reset/update_rules lifecycle in
// src/muz/pdr/pdr_dl_interface.cpp.

import (
	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) advanceRuleRevision() {
	// Rule changes cannot hide an earlier background or parameter change.
	if e.pdrRuleRevision == e.revision {
		e.pdrRuleRevision++
	}
	e.revision++
}

func (e *PropositionalEngine) checkPDRSourceRules() error {
	if e.pdr == nil || e.pdrSourceRules == nil || e.pdrRuleRevision != e.revision {
		return nil
	}
	// Source check_reset compares persistent rules before reaching the fresh
	// synthetic query rule. Report an earlier uncovered persistent rule in the
	// same order; the fresh query rule itself guarantees the later reset.
	missing, err := firstUncoveredHornRule(e.pdrSourceRules, e.set)
	if err != nil {
		return err
	}
	if missing >= 0 {
		oldCount, err := e.pdrSourceRules.Len()
		if err == nil && oldCount > 0 {
			err = e.tracePDRFreshRule(missing)
		}
		return err
	}
	return nil
}

// replacePDRRuleSnapshots publishes the new query snapshots only after every
// superseded owner has completed native cleanup. A refused close leaves the
// old concrete owner reachable through the engine for retry.
func (e *PropositionalEngine) replacePDRRuleSnapshots(rules, source *base.RuleSet) error {
	if e.pdrRules != nil && e.pdrRules != rules {
		if err := e.pdrRules.Close(); err != nil {
			return err
		}
	}
	if e.pdrSourceRules != nil && e.pdrSourceRules != source {
		if err := e.pdrSourceRules.Close(); err != nil {
			return err
		}
	}
	e.pdrRules = rules
	e.pdrSourceRules = source
	return nil
}
