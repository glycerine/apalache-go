package control

// Concrete reslimit specialization of src/util/event_handler.h/cancel_eh.h.

type EventCaller uint32

const (
	EventCallerUnset EventCaller = iota
	EventCallerControlC
	EventCallerTimeout
	EventCallerResourceLimit
	EventCallerAPIInterrupt
)

type utilEventCaller = EventCaller

const (
	utilUnsetCaller         = EventCallerUnset
	utilControlCCaller      = EventCallerControlC
	utilTimeoutCaller       = EventCallerTimeout
	utilResourceLimitCaller = EventCallerResourceLimit
	utilAPIInterruptCaller  = EventCallerAPIInterrupt
)

// This cold scope borrows its resource owner. Invoke and Close must be serialized
// by the caller, as in the source event handler. The underlying cancel counter
// supports asynchronous cancellation. Do not copy an active handler or close its
// owner first. The handler has no interface dispatch or type parameters.
type utilCancelHandler struct {
	owner               *Reslimit
	caller              utilEventCaller
	wasCanceled, closed bool
}

func newUtilCancelHandler(r *Reslimit) utilCancelHandler {
	r.CheckOpen()
	return utilCancelHandler{owner: r}
}
func (h *utilCancelHandler) callerID() utilEventCaller { return h.caller }
func (h *utilCancelHandler) canceled() bool            { return h.wasCanceled }
func (h *utilCancelHandler) invoke(caller utilEventCaller) {
	if h.closed {
		panic("g3: closed cancel handler")
	}
	if !h.wasCanceled {
		h.caller = caller
		h.wasCanceled = true
		h.owner.IncCancel()
	}
}

// Explicit deferred close replaces the single source RAII destructor. Repeated
// close is harmless and does not decrement another scope's cancellation.
func (h *utilCancelHandler) close() {
	if h.closed {
		return
	}
	if h.wasCanceled {
		h.owner.DecCancel()
	}
	h.closed = true
}
