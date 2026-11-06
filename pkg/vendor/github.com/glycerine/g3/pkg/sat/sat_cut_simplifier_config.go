package sat

type satCutSimplifierConfig struct{ enableUnits, enableDontCares, learnImplies, learnedToAIG, validateCuts, validateLemmas, simulateEquivalences bool }

func defaultSATCutSimplifierConfig() satCutSimplifierConfig {
	return satCutSimplifierConfig{enableUnits: true, enableDontCares: true, learnedToAIG: true}
}
