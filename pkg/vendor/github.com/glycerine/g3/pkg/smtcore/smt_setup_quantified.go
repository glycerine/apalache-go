package smtcore

// Source: src/smt/smt_setup.cpp setup_AUFLIA, setup_AUFLIRA,
// setup_UFNIA, setup_AUFNIRA, setup_UFLRA and quantified setup_unknown.

import (
	"errors"
	"fmt"
)

func (k *SMTKernel) setupQuantifiedLogicPolicy(useStaticFeatures bool) (result error) {
	mixed, simpleArray, inspect := false, true, false
	switch k.setupLogic {
	case "AUFLIA":
		inspect = useStaticFeatures
	case "AUFLIA+", "AUFLIA-", "UFNIA":
	case "AUFLIRA", "AUFNIRA", "AUFLIRA+", "AUFLIRA-", "AUFNIRA+", "AUFNIRA-":
		mixed = true
	case "UFLRA":
		mixed, inspect = true, useStaticFeatures
		simpleArray = !useStaticFeatures
	default:
		if !useStaticFeatures || !smtLogicUsesUnknownSetup(k.setupLogic) {
			return nil
		}
		inspect, simpleArray = true, false
	}
	features, err := k.setupStaticFeatures(inspect)
	if err != nil {
		return err
	}
	if features != nil {
		defer func() { result = errors.Join(result, features.Close()) }()
		if !simpleArray {
			if features.NumQuantifiers == 0 {
				return nil
			}
			mixed = features.HasReal
		} else if features.HasReal {
			return fmt.Errorf("Benchmark has real variables but it is marked as AUFLIA (arrays, uninterpreted functions and linear integer arithmetic).")
		}
	}
	qi := &k.smtParams.qi.state
	preprocess := &k.smtParams.preprocess.state
	pattern := &k.smtParams.preprocess.pattern.state
	array := &k.smtParams.array.state
	array.Mode = 1 // AR_SIMPLE
	if !simpleArray {
		array.Mode = 3 // AR_FULL
	}
	k.smtParams.state.PhaseSelection = 0 // PS_ALWAYS_FALSE
	preprocess.EliminateBounds = true
	preprocess.PropagateBooleans = true
	preprocess.NonGroundLiftITE = smtLiftITEFull
	qi.QuickChecker = 1 // MC_UNSAT
	qi.LazyThreshold = 20
	qi.MBQI = true
	if mixed {
		qi.EagerThreshold = 5
		preprocess.MacroFinder = true
		pattern.MaxMultiPatterns = 10
		array.LazyIEq = true
		array.LazyIEqDelay = 4
	} else {
		pattern.UseDatabase = true
		k.smtParams.state.RestartStrategy = 0 // RS_GEOMETRIC
		k.smtParams.state.RestartFactor = 1.5
		if simpleArray && features != nil {
			qi.EagerThreshold = 5
			if features.NumQuantifiersWithPatterns != 0 {
				qi.EagerThreshold = 7
			}
		}
	}
	if k.patternInference != nil {
		if err = k.smtParams.preprocess.pattern.UpdateInferenceRewriter(k.patternInference, nil); err != nil {
			return err
		}
	}
	if manager := k.internalizer.quantifierManager; manager != nil {
		identifier, err := qi.MBQIId.String()
		if err != nil {
			return err
		}
		manager.mbqiTrace = qi.Trace
		if err = manager.UpdateMBQI(qi.MBQI, identifier, qi.MaxCexs, qi.MaxIterations, qi.ForceTemplate); err != nil {
			return err
		}
		manager.quickUnsat, manager.quickNotSat = true, false
		manager.registry.schedule.lazyThreshold = float32(qi.LazyThreshold)
		// quantifier_manager::imp calls qi_queue::setup in its constructor.
		// Its eager threshold and parsed expressions stay cached even when
		// logic setup subsequently changes the owning parameter fields.
	}
	if mixed {
		if err = k.installMixedArithmeticSetup(k.smtParams.arith.state); err != nil {
			return err
		}
	}
	return k.applySimpleSetupPolicy()
}
