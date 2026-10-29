package params

// Source: src/util/gparams.cpp:555,633-644; generated mem_initialize/finalize.

// The source gparams namespace has one process-wide current service. Generated
// initialization publishes the concrete Go owner here after gparams::init;
// generated finalization clears its Current pointer, and owning Close restores
// the prior binding. The port's current process path is single-goroutine.
var processGlobalParamsService *GlobalParamsService

func BorrowProcessGlobalParamsService() *GlobalParamsService {
	return processGlobalParamsService
}

func BindProcessGlobalParamsService(s *GlobalParamsService) (previous *GlobalParamsService, err error) {
	if s != nil {
		if err = s.open(); err != nil {
			return nil, err
		}
	}
	previous = processGlobalParamsService
	processGlobalParamsService = s
	return previous, nil
}
