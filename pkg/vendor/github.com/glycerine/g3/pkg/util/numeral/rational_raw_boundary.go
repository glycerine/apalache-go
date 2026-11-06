package numeral

// Source: src/util/mpq.h/.cpp: caller observations of actual RAW invocations.

type RationalRawInvocationState struct {
	Active      [3]bool
	Generations [3]uint64
}

func (m *RationalManager) RawInvocationState() RationalRawInvocationState {
	r := &m.RawArithmetic
	return RationalRawInvocationState{[3]bool{r.NormActive, r.AddActive, r.PowerActive}, [3]uint64{r.normGeneration, r.addGeneration, r.PowerGeneration}}
}
func (m *RationalManager) CleanupRawInvocations(saved *[3]uint64) error {
	state := m.RawInvocationState()
	groups := [3]rationalRawGroup{rationalRawNormalize, rationalRawAddition, RationalRawPower}
	for i, g := range *saved {
		if g != 0 && g == state.Generations[i] {
			if e := m.CleanupRawGroup(groups[i]); e != nil {
				return e
			}
		}
		saved[i] = 0
	}
	return nil
}

// RationalComparisonInvocationState identifies the actual synchronized raw
// cross-product invocation. It is separate from numeral values and does not
// infer ownership from allocated cells or numeral values.
type RationalComparisonInvocationState struct {
	Active     bool
	Generation uint64
}

func (m *RationalManager) ComparisonInvocationState() RationalComparisonInvocationState {
	r := &m.RawArithmetic
	return RationalComparisonInvocationState{Active: r.CompareActive, Generation: r.compareGeneration}
}

// CleanupComparisonInvocation releases a matching caller-acquired generation.
// Older and replaced generations are left to their actual invoking owners.
func (m *RationalManager) CleanupComparisonInvocation(saved *uint64) error {
	state := m.ComparisonInvocationState()
	if *saved != 0 && *saved == state.Generation {
		if err := m.CleanupRawGroup(rationalRawComparison); err != nil {
			return err
		}
	}
	*saved = 0
	return nil
}

// RationalDivisionInvocationState identifies the actual divisor/destination
// alias temporary. A receipt never grants ownership of another caller's frame.
type RationalDivisionInvocationState struct {
	Active     bool
	Generation uint64
}

func (m *RationalManager) DivisionInvocationState() RationalDivisionInvocationState {
	r := &m.RawArithmetic
	return RationalDivisionInvocationState{Active: r.DivActive, Generation: r.divGeneration}
}

func (m *RationalManager) CleanupDivisionInvocation(saved *uint64) error {
	state := m.DivisionInvocationState()
	if *saved != 0 && *saved == state.Generation {
		if err := m.CleanupRawGroup(rationalRawDivision); err != nil {
			return err
		}
	}
	*saved = 0
	return nil
}
