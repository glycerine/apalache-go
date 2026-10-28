package apalache

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CheckerTuning is the validated Scala search-tuning contract used by the Go
// checker. Regular expressions use full matches, as
// java.util.regex.Matcher.matches does in Scala.
type CheckerTuning struct {
	transitionFilter *regexp.Regexp
	invariantFilter  *regexp.Regexp
	// Explicit path enumeration has no joined transition relation, so the
	// Scala before/after modes have identical behavior after validation.
	invariantMode     string
	shortCircuit      bool
	solverUIntOptions map[string]uint32
	solverTextOptions map[string]string
	outputTraces      bool
	hasOutputTraces   bool
	simulation        bool
	simulationRuns    int
	hasSimulation     bool
	jailTimeoutMinSec int64
	jailTimeoutFactor int64
	idleTimeoutSec    int64
}

// ParseCheckerTuning validates checker tuning accepted by the Go backend.
// Solver-specific options are accepted only when the pinned g3 API exposes a
// compatible typed parameter rather than being silently ignored.
func ParseCheckerTuning(values map[string]string) (CheckerTuning, error) {
	result := CheckerTuning{
		invariantMode: "before", solverUIntOptions: make(map[string]uint32), solverTextOptions: make(map[string]string),
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		switch key {
		case "search.transitionFilter":
			if value == "" {
				continue
			}
			compiled, err := regexp.Compile("^(?:" + value + ")$")
			if err != nil {
				return CheckerTuning{}, fmt.Errorf("invalid regex %s: %w", value, err)
			}
			result.transitionFilter = compiled
		case "search.invariantFilter":
			if value == "" {
				continue
			}
			compiled, err := regexp.Compile("^(?:" + value + ")$")
			if err != nil {
				return CheckerTuning{}, fmt.Errorf("invalid regex %s: %w", value, err)
			}
			result.invariantFilter = compiled
		case "search.invariant.mode":
			if value != "before" && value != "after" {
				return CheckerTuning{}, fmt.Errorf("invalid value %s for %s, expected one of before, after", value, key)
			}
			result.invariantMode = value
		case "rewriter.shortCircuit":
			parsed, err := parseTuningBool(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.shortCircuit = parsed
		case "search.simulation":
			parsed, err := parseTuningBool(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.simulation = parsed
			result.hasSimulation = true
		case "search.simulation.maxRun":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return CheckerTuning{}, fmt.Errorf("expected a positive integer for %s, but got %s", key, value)
			}
			result.simulationRuns = parsed
			result.hasSimulation = true
		case "search.outputTraces":
			parsed, err := parseTuningBool(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.outputTraces = parsed
			result.hasOutputTraces = true
		case "search.split.timeout.minimum":
			parsed, err := parseTuningInt64(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.jailTimeoutMinSec = parsed
		case "search.split.timeout.factor":
			parsed, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return CheckerTuning{}, fmt.Errorf("expected a 32-bit integer for %s, but got %s", key, value)
			}
			result.jailTimeoutFactor = parsed
		case "search.idle.timeout":
			parsed, err := parseTuningInt64(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.idleTimeoutSec = parsed
		case "smt.randomSeed":
			parsed, err := parseTuningUInt(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.solverUIntOptions["sat.random_seed"] = parsed
			result.solverUIntOptions["smt.random_seed"] = parsed
		case "z3.smt.random_seed", "z3.sat.random_seed", "z3.timeout", "z3.rlimit":
			parsed, err := parseTuningUInt(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.solverUIntOptions[key[len("z3."):]] = parsed
		case "z3.memory_high_watermark_mb":
			parsed, err := parseTuningUInt(key, value)
			if err != nil {
				return CheckerTuning{}, err
			}
			result.solverUIntOptions["memory_high_watermark"] = parsed
		default:
			if strings.HasPrefix(key, "z3.") {
				result.solverTextOptions[key[len("z3."):]] = value
			} else {
				return CheckerTuning{}, fmt.Errorf("unexpected tuning field %s", key)
			}
		}
	}
	if len(result.solverTextOptions) != 0 {
		solver, err := newG3SolverContextWithOptions("QF_AUFNIA", result.solverUIntOptions, result.solverTextOptions)
		if err != nil {
			return CheckerTuning{}, fmt.Errorf("invalid g3 tuning: %w", err)
		}
		if err = solver.Close(); err != nil {
			return CheckerTuning{}, err
		}
	}
	return result, nil
}

func parseTuningBool(key, value string) (bool, error) {
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}
	return false, fmt.Errorf("non-Boolean value %s for %s", value, key)
}

func parseTuningInt64(key, value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("expected a 64-bit integer for %s, but got %s", key, value)
	}
	return parsed, nil
}

func (tuning CheckerTuning) SimulationRuns() int {
	if tuning.simulationRuns > 0 {
		return tuning.simulationRuns
	}
	return 100
}

func (tuning CheckerTuning) HasSimulationControls() bool { return tuning.hasSimulation }

func (tuning CheckerTuning) OutputTraces() bool { return tuning.outputTraces }

func (tuning CheckerTuning) HasOutputTraces() bool { return tuning.hasOutputTraces }

func (tuning CheckerTuning) WithOutputTraces(enabled bool) CheckerTuning {
	tuning.outputTraces = enabled
	tuning.hasOutputTraces = true
	return tuning
}

func (tuning CheckerTuning) WithSMTRandomSeed(seed uint32) CheckerTuning {
	tuning.solverUIntOptions = cloneMap(tuning.solverUIntOptions)
	if tuning.solverUIntOptions == nil {
		tuning.solverUIntOptions = make(map[string]uint32)
	}
	tuning.solverUIntOptions["sat.random_seed"] = seed
	tuning.solverUIntOptions["smt.random_seed"] = seed
	return tuning
}

func (tuning CheckerTuning) withExactTransitionPath(init int, next []int) CheckerTuning {
	words := make([]string, 0, len(next)+1)
	words = append(words, fmt.Sprintf("0->%d", init))
	for depth, number := range next {
		words = append(words, fmt.Sprintf("%d->%d", depth+1, number))
	}
	copy := tuning
	copy.transitionFilter = regexp.MustCompile("^(?:" + strings.Join(words, "|") + ")$")
	return copy
}

func parseTuningUInt(key, value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed > math.MaxInt32 {
		return 0, fmt.Errorf("expected an unsigned integer for %s, but got %s", key, value)
	}
	return uint32(parsed), nil
}

func (tuning CheckerTuning) allowsTransition(step, number int) bool {
	return tuning.transitionFilter == nil || tuning.transitionFilter.MatchString(fmt.Sprintf("%d->%d", step, number))
}

func (tuning CheckerTuning) allowsInvariant(step int, kind string, number int) bool {
	return tuning.invariantFilter == nil || tuning.invariantFilter.MatchString(fmt.Sprintf("%d->%s%d", step, kind, number))
}
