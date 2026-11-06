package api

// Source: cancel_eh<reslimit> used by src/api/api_ast.cpp:679-696.

import "github.com/glycerine/g3/pkg/util/control"

type smtContextCancelHandler struct {
	limit  *control.Reslimit
	caller control.EventCaller
	active bool
}

func newSMTContextCancelHandler(limit *control.Reslimit) *smtContextCancelHandler {
	return &smtContextCancelHandler{limit: limit, active: limit != nil}
}

func (h *smtContextCancelHandler) HandleEvent(caller control.EventCaller) {
	if h == nil || !h.active || h.limit == nil {
		return
	}
	h.caller = caller
	h.limit.Cancel()
}

func (h *smtContextCancelHandler) close() {
	if h == nil {
		return
	}
	h.active = false
	h.limit = nil
}
