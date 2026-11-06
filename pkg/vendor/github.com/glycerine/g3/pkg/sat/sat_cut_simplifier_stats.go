package sat

type satCutSimplifierStats struct{ numEquivalences, numUnits, numCuts, numXors, numAnds, numITEs, extractedXors, extractedAnds, extractedITEs, extractedLUTs, numCalls, numDontCareReductions, numLearnedImplies uint32 }
