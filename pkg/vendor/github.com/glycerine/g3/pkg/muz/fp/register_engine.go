package fp

// Source: src/muz/fp/dl_register_engine.cpp concrete fixedpoint engine factory.

import "fmt"

func (e *PropositionalEngine) selectBackend(kind EngineKind) error {
	if e.backend != kind.String() {
		e.invalidateQueryForMutation()
		if err := e.closeExternalIntervalDatalog(); err != nil {
			return err
		}
		if err := e.closeSparseIntervalDatalog(); err != nil {
			return err
		}
		if err := e.closeIntervalDatalog(); err != nil {
			return err
		}
		if err := e.closeBoundDatalog(); err != nil {
			return err
		}
		if err := e.closeBoundIntervalDatalog(); err != nil {
			return err
		}
		if err := e.closeDuality(); err != nil {
			return err
		}
		if err := e.clearQueryResources(); err != nil {
			return err
		}
		if err := e.closeUDocDatalog(); err != nil {
			return err
		}
		if e.backend == EngineBMC.String() || e.backend == EngineQBMC.String() {
			if err := e.clearBMCTrace(); err != nil {
				return err
			}
			e.bmcStatistics.Reset()
		}
		if e.backend == EngineTab.String() {
			if err := e.clearTabAnswer(); err != nil {
				return err
			}
			if err := e.closeTabSession(); err != nil {
				return err
			}
			e.tabUnfold, e.tabUnfoldFail, e.tabSubsumed = 0, 0, 0
		}
		if e.backend == EngineDDNF.String() {
			if err := e.ddnf.closeTrail(e.manager); err != nil {
				return err
			}
			if e.ddnf.registry != nil && !e.ddnf.registryBorrowed {
				e.ddnf.registry.Close()
			}
			e.ddnf = ddnfEngineState{output: e.ddnf.output}
		}
	}
	if e.backend != kind.String() {
		e.engineInstantiated = false
		e.datalogDefaultTable = ""
		e.datalogRelationFavourite = ""
	}
	e.backend = kind.String()
	e.engineSelectionSet = true
	return nil
}

// SelectEngine applies the retained fixedpoint engine factory branch.
func (e *PropositionalEngine) SelectEngine(name string) error {
	if err := e.open(); err != nil {
		return err
	}
	if name == "auto-config" {
		kind, err := e.autoEngineKind()
		if err != nil {
			return err
		}
		if kind != EngineDatalog && (e.params == nil || e.globals == nil) {
			return fmt.Errorf("muz/fp: fixedpoint auto-config requires parameter services for %s", kind)
		}
		return e.selectBackend(kind)
	}
	kind, err := parseEngineKind(name)
	if err != nil {
		return err
	}
	if !kind.Retained() {
		return fmt.Errorf("muz/fp: fixedpoint engine %s is not ported", name)
	}
	switch kind {
	case EngineDatalog:
		return e.selectBackend(kind)
	case EnginePDR, EngineQPDR, EngineSpacer, EngineBMC, EngineQBMC, EngineTab, EngineCLP, EngineDuality:
		if e.params == nil || e.globals == nil {
			return fmt.Errorf("muz/fp: fixedpoint engine %s requires parameter services", name)
		}
		return e.selectBackend(kind)
	case EngineDDNF:
		return e.selectBackend(kind)
	}
	return fmt.Errorf("muz/fp: fixedpoint engine %s is not ported", name)
}
