package arith

// Source: src/tactic/arith/fm_tactic.cpp:786-793,1450-1470.
type FMEliminationConfig struct {
	CutoffSide    uint32
	CutoffProduct uint32
	Extra         uint32
}

func DefaultFMEliminationConfig() FMEliminationConfig {
	return FMEliminationConfig{CutoffSide: 8, CutoffProduct: 256}
}

func (c FMEliminationConfig) permits(lowers, uppers int) bool {
	if uint32(lowers) > c.CutoffSide && uint32(uppers) > c.CutoffSide {
		return false
	}
	return uint32(lowers)*uint32(uppers) <= c.CutoffProduct
}
