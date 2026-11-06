package fp

// Source: src/muz/base/dl_engine_base.h result contract,
// src/muz/pdr/pdr_dl_interface.{h,cpp}, and
// src/muz/spacer/spacer_dl_interface.{h,cpp}.

import (
	"strings"

	muzspacer "github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func fixedpointFailureStatus(cause error) string {
	if cause == nil {
		return "ok"
	}
	message := cause.Error()
	if strings.Contains(message, messages.MaxMemory) {
		return "memory bounds exceeded"
	}
	if strings.Contains(message, messages.Canceled) || strings.Contains(message, messages.MaxResource) || strings.Contains(message, messages.MaxSteps) {
		return "canceled"
	}
	return "input error"
}

// ResetCommandStatus performs the status-reset portion of context::cleanup
// after an SMT-LIB query command has printed its result and certificate.
func (e *PropositionalEngine) ResetCommandStatus() error {
	if err := e.open(); err != nil {
		return err
	}
	e.lastStatus = "ok"
	return nil
}

// recordResultSnapshot publishes a result only after its query returned. The
// revision and engine kind prevent stale models/certificates from escaping
// after rules, parameters, or engine selection change.
func (e *PropositionalEngine) recordResultSnapshot(result lbool.LBool, queryErr error) {
	if queryErr != nil {
		e.queryReady = false
		e.lastResult = lbool.Undef
		return
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil || !kind.Retained() {
		e.queryReady = false
		e.lastResult = lbool.Undef
		return
	}
	e.queryReady = true
	e.queryRevision = e.revision
	e.queryEngine = kind
	e.lastResult = result
}

func (e *PropositionalEngine) recordQueryStatusWithApproximation(result lbool.LBool, err error, approximate bool) {
	e.recordResultSnapshot(result, err)
	if err != nil {
		e.lastStatus = fixedpointFailureStatus(err)
		return
	}
	if result == lbool.Undef {
		// This path represents an approximation without dispatching a backend
		// query, so it must not inherit a bounded state from an earlier Spacer
		// search.
		if approximate {
			e.lastStatus = "approximated"
			return
		}
		var context *muzspacer.Context
		if e.backend == "spacer" {
			context = e.spacer
			if context == nil && e.queryClone != nil && e.queryClone.backend == "spacer" {
				context = e.queryClone.spacer
			}
		}
		if context != nil {
			bounded, boundedErr := context.Bounded()
			if boundedErr == nil && bounded {
				e.lastStatus = "bounded"
				return
			}
		}
		// Source context::query resets the execution status to OK before
		// dispatch. Only relational Datalog marks an imprecise result APPROX;
		// CLP, TAB, BMC/QBMC, DDNF and ordinary PDR/Spacer unknowns leave it
		// unchanged. The explicit flag preserves the public multi-relation
		// non-Datalog extension, which is intentionally approximate.
		if e.backend == "datalog" {
			e.lastStatus = "approximated"
		} else {
			e.lastStatus = "ok"
		}
		return
	}
	e.lastStatus = "ok"
}

// Parameter/engine mutation invalidates result and property snapshots before
// disposing their owners. Failed solves keep their separate property revision.
func (e *PropositionalEngine) invalidateQueryForMutation() {
	e.queryReady = false
	e.lastResult = lbool.Undef
	e.queryCloneRevision = 0
}

func (e *PropositionalEngine) requireResultSnapshot(kind EngineKind) error {
	if err := e.open(); err != nil {
		return err
	}
	selected, err := parseEngineKind(e.backend)
	if err != nil || selected != kind || !e.queried || !e.queryReady || e.queryEngine != kind || e.queryRevision != e.revision {
		return ErrPropositionalEngine
	}
	switch kind {
	case EngineDuality:
		if e.duality == nil {
			return ErrPropositionalEngine
		}
	case EnginePDR, EngineQPDR:
		if e.pdr == nil || e.pdrRevision != e.revision {
			return ErrPropositionalEngine
		}
	case EngineSpacer:
		if e.spacer == nil || e.spacerRevision != e.revision {
			return ErrPropositionalEngine
		}
	}
	return nil
}

func (e *PropositionalEngine) requirePDRSnapshot() error {
	kind, err := parseEngineKind(e.backend)
	if err != nil || (kind != EnginePDR && kind != EngineQPDR) {
		return ErrPropositionalEngine
	}
	return e.requireResultSnapshot(kind)
}

func (e *PropositionalEngine) requireBMCSnapshot() error {
	kind, err := parseEngineKind(e.backend)
	if err != nil || (kind != EngineBMC && kind != EngineQBMC) {
		return ErrPropositionalEngine
	}
	return e.requireResultSnapshot(kind)
}

// LastResult returns the retained source result for the latest non-stale
// query, including unknown. Mutations invalidate the snapshot.
func (e *PropositionalEngine) LastResult() (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil {
		return lbool.Undef, err
	}
	if e.queryClone != nil {
		if _, _, err = e.expressionQueryState(); err != nil {
			return lbool.Undef, err
		}
		return e.lastResult, nil
	}
	if err = e.requireResultSnapshot(kind); err != nil {
		return lbool.Undef, err
	}
	return e.lastResult, nil
}
