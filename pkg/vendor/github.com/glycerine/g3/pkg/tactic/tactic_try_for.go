package tactic

// Source: src/tactic/tactical.cpp:1130-1156.

import (
	"errors"
	"sync"

	"github.com/glycerine/g3/pkg/util/control"
)

type tacticTimeoutHandler struct {
	mu     sync.Mutex
	limit  *control.Reslimit
	active bool
	fired  bool
}

func (h *tacticTimeoutHandler) HandleEvent(control.EventCaller) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active || h.fired {
		return
	}
	h.limit.IncCancel()
	h.fired = true
}

func (h *tacticTimeoutHandler) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = false
	if h.fired {
		h.limit.DecCancel()
	}
}

func (t *unaryTactic) applyTryFor(g *Goal, out *TacticResult) (result error) {
	limit := g.manager.ResourceLimit()
	handler := &tacticTimeoutHandler{limit: limit, active: true}
	timer, err := control.NewScopedTimer(t.threshold, handler)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, timer.Close())
		handler.close()
	}()
	return t.child.Apply(g, out)
}
