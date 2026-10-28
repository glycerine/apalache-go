package apalache

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type SMTEncoding string

const (
	SMTEncodingOOPSLA19  SMTEncoding = "oopsla19"
	SMTEncodingArrays    SMTEncoding = "arrays"
	SMTEncodingFunArrays SMTEncoding = "fun-arrays"
)

func ParseSMTEncoding(value string) (SMTEncoding, error) {
	switch SMTEncoding(value) {
	case SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays:
		return SMTEncoding(value), nil
	default:
		return "", fmt.Errorf("unexpected SMT encoding: %s", value)
	}
}

type SMTSolverBackend string

const (
	SMTSolverZ3   SMTSolverBackend = "z3"
	SMTSolverCVC5 SMTSolverBackend = "cvc5"
)

func ParseSMTSolverBackend(value string) (SMTSolverBackend, error) {
	switch SMTSolverBackend(strings.ToLower(value)) {
	case SMTSolverZ3:
		return SMTSolverZ3, nil
	case SMTSolverCVC5:
		return SMTSolverCVC5, nil
	default:
		return "", fmt.Errorf("unexpected SMT solver backend: %s", strings.ToLower(value))
	}
}

// SolverConfig retains Scala's solver configuration contract. The standalone
// port executes its Z3-compatible backend through pinned pure-Go g3.
type SolverConfig struct {
	Debug          bool
	Profile        bool
	RandomSeed     int
	SMTEncoding    SMTEncoding
	Z3StatsSeconds int
	SMTSolver      SMTSolverBackend
	solverParams   map[string]any
}

func DefaultSolverConfig() SolverConfig {
	return SolverConfig{
		SMTEncoding: SMTEncodingOOPSLA19, Z3StatsSeconds: 60, SMTSolver: SMTSolverZ3,
		solverParams: make(map[string]any),
	}
}

func NewSolverConfig(
	debug bool,
	profile bool,
	randomSeed int,
	encoding SMTEncoding,
	z3StatsSeconds int,
	solver SMTSolverBackend,
	parameters map[string]any,
) SolverConfig {
	return SolverConfig{
		Debug: debug, Profile: profile, RandomSeed: randomSeed,
		SMTEncoding: encoding, Z3StatsSeconds: z3StatsSeconds, SMTSolver: solver,
		solverParams: cloneMap(parameters),
	}
}

func (config SolverConfig) SolverParameters() map[string]any {
	return cloneMap(config.solverParams)
}

func (config SolverConfig) WithSolverParameters(parameters map[string]any) SolverConfig {
	config.solverParams = cloneMap(parameters)
	return config
}

func NewG3SolverContextWithConfig(config SolverConfig) (*G3SolverContext, error) {
	if config.SMTEncoding != SMTEncodingOOPSLA19 {
		return nil, fmt.Errorf("g3 supports SMT encoding %s, found %s", SMTEncodingOOPSLA19, config.SMTEncoding)
	}
	if config.SMTSolver != SMTSolverZ3 {
		return nil, fmt.Errorf("g3 is the Z3-compatible backend, found %s", config.SMTSolver)
	}
	if config.RandomSeed < 0 || uint64(config.RandomSeed) > math.MaxInt32 {
		return nil, fmt.Errorf("solver random seed must be in 0..%d, found %d", math.MaxInt32, config.RandomSeed)
	}
	uintOptions := make(map[string]uint32)
	textOptions := make(map[string]string)
	if config.RandomSeed != 0 {
		uintOptions["sat.random_seed"] = uint32(config.RandomSeed)
		uintOptions["smt.random_seed"] = uint32(config.RandomSeed)
	}
	for key, value := range config.solverParams {
		key = strings.TrimPrefix(key, "z3.")
		if key == "" {
			return nil, fmt.Errorf("solver parameter name cannot be empty")
		}
		switch typed := value.(type) {
		case uint32:
			uintOptions[key] = typed
		case uint:
			if uint64(typed) > math.MaxInt32 {
				return nil, fmt.Errorf("solver parameter %s is out of range: %d", key, typed)
			}
			uintOptions[key] = uint32(typed)
		case int:
			if typed < 0 || uint64(typed) > math.MaxInt32 {
				return nil, fmt.Errorf("solver parameter %s is out of range: %d", key, typed)
			}
			uintOptions[key] = uint32(typed)
		case int64:
			if typed < 0 || uint64(typed) > math.MaxInt32 {
				return nil, fmt.Errorf("solver parameter %s is out of range: %d", key, typed)
			}
			uintOptions[key] = uint32(typed)
		case bool:
			textOptions[key] = strconv.FormatBool(typed)
		case string:
			textOptions[key] = typed
		case float64:
			textOptions[key] = strconv.FormatFloat(typed, 'g', -1, 64)
		default:
			return nil, fmt.Errorf("solver parameter %s has unsupported value type %T", key, value)
		}
	}
	return newG3SolverContextWithOptions("QF_AUFNIA", uintOptions, textOptions)
}
