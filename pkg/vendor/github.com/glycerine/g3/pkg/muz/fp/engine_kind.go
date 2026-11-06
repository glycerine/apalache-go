package fp

// Source: src/muz/base/dl_engine_base.h DL_ENGINE and
// src/muz/fp/dl_register_engine.{h,cpp}.

import "fmt"

// EngineKind is the source fixedpoint-engine selection domain. Only retained
// engines can be selected; the remaining names are represented so diagnostics
// distinguish a known unported engine from an unknown spelling.
type EngineKind uint8

const (
	EngineDatalog EngineKind = iota
	EnginePDR
	EngineSpacer
	EngineQPDR
	EngineBMC
	EngineQBMC
	EngineTab
	EngineCLP
	EngineDuality
	EngineDDNF
)

var engineNames = [...]string{
	"datalog", "pdr", "spacer", "qpdr", "bmc", "qbmc", "tab", "clp", "duality", "ddnf",
}

func (k EngineKind) String() string {
	if int(k) >= len(engineNames) {
		return "unknown"
	}
	return engineNames[k]
}

func (k EngineKind) Retained() bool {
	return k == EngineDatalog || k == EnginePDR || k == EngineQPDR || k == EngineSpacer || k == EngineBMC || k == EngineQBMC || k == EngineTab || k == EngineCLP || k == EngineDDNF || k == EngineDuality
}

func parseEngineKind(name string) (EngineKind, error) {
	for kind, candidate := range engineNames {
		if name == candidate {
			return EngineKind(kind), nil
		}
	}
	return EngineDatalog, fmt.Errorf("muz/fp: unknown fixedpoint engine %s", name)
}

// SelectedEngine returns the concrete retained source engine branch.
func (e *PropositionalEngine) SelectedEngine() (EngineKind, error) {
	if err := e.open(); err != nil {
		return EngineDatalog, err
	}
	return parseEngineKind(e.backend)
}
