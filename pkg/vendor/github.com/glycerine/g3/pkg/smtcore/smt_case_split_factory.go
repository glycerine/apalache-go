package smtcore

// Source: src/smt/smt_case_split_queue.cpp:1229-1262 mk_case_split_queue.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// The factory mutates its borrowed fparams before allocating the queue. The
// sequential tests matter: the relevancy fallback suppresses the auto-config
// warning by changing the strategy before that second test.
func normalizeSMTCaseSplitFactory(params *SMTParamsState, warnings *diagnostic.WarningService) error {
	if warnings == nil {
		warnings = diagnostic.BorrowProcessWarningService()
	}
	if params.RelevancyLvl < 2 && params.CaseSplitStrategy >= 3 && params.CaseSplitStrategy <= 5 {
		if err := warnings.Warning("relevancy must be enabled to use option CASE_SPLIT=3, 4 or 5"); err != nil {
			return err
		}
		params.CaseSplitStrategy = 0
	}
	if params.AutoConfig && params.CaseSplitStrategy >= 3 && params.CaseSplitStrategy <= 5 {
		if err := warnings.Warning("auto configuration (option AUTO_CONFIG) must be disabled to use option CASE_SPLIT=3, 4 or 5"); err != nil {
			return err
		}
		params.CaseSplitStrategy = 0
	}
	return nil
}
