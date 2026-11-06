package api

// Source: src/api/api_context.cpp:364-368.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// ToggleWarningMessages sets the process warning gate used by API callers.
func ToggleWarningMessages(enabled bool) {
	log := enterZ3APILog()
	defer leaveZ3APILog(log)
	if log != nil {
		_ = log.logToggleWarnings(enabled)
	}
	_ = diagnostic.BorrowProcessWarningService().Enable(enabled)
}
