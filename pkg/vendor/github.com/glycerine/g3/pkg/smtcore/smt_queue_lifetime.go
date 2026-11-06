package smtcore

// Source: src/smt/smt_context.cpp:44-92,135-138,3281-3293,3367-3394;
// src/smt/smt_case_split_queue.cpp mk_case_split_queue and borrowed m_params.

func effectiveSMTCaseSplitStrategy(params SMTParamsState) uint32 {
	strategy := params.CaseSplitStrategy
	if strategy > 6 || strategy >= 3 && strategy <= 5 && (params.RelevancyLvl < 2 || params.AutoConfig) {
		return 0
	}
	return strategy
}

// The constructor receives the actual fparams field, so source factory
// mutations survive a queue-allocation refusal. RNG construction precedes
// warning callbacks; those callbacks must not cause a second constructor seed.
func (k *SMTKernel) initializeQueueHeuristics(params *SMTParamsState) error {
	k.internalizer.random.SetSeed(params.RandomSeed)
	if err := normalizeSMTCaseSplitFactory(params, k.queueWarnings); err != nil {
		return err
	}
	strategy := effectiveSMTCaseSplitStrategy(*params)
	if err := k.internalizer.configureBooleanHeuristicsForQueue(*params, strategy, false); err != nil {
		return err
	}
	queue, err := NewSMTActivityCaseSplitQueue(k.internalizer)
	if err != nil {
		return err
	}
	if err = queue.ConfigurePhases(*params); err != nil {
		return err
	}
	k.caseSplitQueue = queue
	queue.liveParams = k.smtParams
	k.smtState.CaseSplitStrategy = params.CaseSplitStrategy
	return nil
}

// Kernel contexts construct their queue once. Live updates change parameters,
// while the installed concrete queue class and phase-cache counters survive.
func (k *SMTKernel) configureQueueHeuristics(params SMTParamsState, setup bool) error {
	if k.caseSplitQueue == nil {
		if err := k.initializeQueueHeuristics(&params); err != nil {
			return err
		}
	}
	queue := k.caseSplitQueue
	if queue.liveParams != nil {
		if err := queue.liveParams.open(); err != nil {
			return err
		}
		params = queue.liveParams.state
	}
	if err := k.internalizer.configureBooleanHeuristicsForQueue(params, queue.strategy, setup); err != nil {
		return err
	}
	return queue.updateLivePhases(&params)
}

func (q *SMTActivityCaseSplitQueue) refreshLivePhases() error {
	if q.liveParams == nil {
		return nil
	}
	if err := q.liveParams.open(); err != nil {
		return err
	}
	return q.updateLivePhases(&q.liveParams.state)
}
