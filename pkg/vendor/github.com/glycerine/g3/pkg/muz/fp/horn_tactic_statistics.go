package fp

// Source: horn_tactic statistics collection/reset and cleanup accumulation.

import "github.com/glycerine/g3/pkg/util/statistics"

func (t *HornTactic) recordEngineStatistics(engine *PropositionalEngine) {
	if engine == nil || t.closed {
		return
	}
	var values statistics.Statistics
	if engine.CollectStatistics(&values) != nil {
		return
	}
	for i := 0; i < values.Size(); i++ {
		var value float64
		if values.IsUint(i) {
			value = float64(values.UintValue(i))
		} else {
			value = values.DoubleValue(i)
		}
		t.statistics[string(values.Key(i))] += value
	}
	t.statistics["applications"]++
}

func (t *HornTactic) CollectStatistics() map[string]float64 {
	result := make(map[string]float64)
	if t == nil || t.closed {
		return result
	}
	for key, value := range t.statistics {
		result[key] = value
	}
	return result
}

func (t *HornTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	clear(t.statistics)
	return nil
}
