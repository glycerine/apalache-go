package base

// Source: src/muz/base/dl_engine_base.h DL_ENGINE.
type EngineKind uint32

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
	EngineKindCount
)
