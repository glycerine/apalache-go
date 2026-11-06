package api

// Source: src/api/z3_replayer.h; src/api/z3_replayer.cpp.

import (
	"bufio"
	"io"
)

type ReplayCommand func(*Z3Replayer) error

type replayCommandEntry struct {
	name string
	call ReplayCommand
}

type Z3Replayer struct {
	scanner              *z3ReplayScanner
	output               io.Writer
	args                 []z3ReplayValue
	result               uint64
	heap                 map[uint64]uint64
	commands             map[uint32]replayCommandEntry
	objectArrays         [][]uint64
	symbolArrays         [][]z3ReplayValue
	uintArrays           [][]uint32
	intArrays            [][]int32
	configs              map[uint64]*Config
	configOrder          []uint64
	contexts             map[uint64]*replayContextOwner
	contextOrder         []uint64
	asts                 map[uint64]*replayASTOwner
	astIDs               map[replayASTKey]uint64
	parameterSets        map[uint64]*replayParameterSet
	descriptions         map[uint64]*replayParameterDescriptions
	solvers              map[uint64]*replaySolver
	models               map[uint64]*replayModel
	goals                map[uint64]*replayGoal
	vectors              map[uint64]*replayASTVector
	functions            map[uint64]*replayFunctionInterpretation
	functionEntries      map[uint64]*replayFunctionEntry
	tactics              map[uint64]*replayTactic
	tacticResults        map[uint64]*replayTacticResult
	probes               map[uint64]*replayProbe
	statistics           map[uint64]*replayStatistics
	maps                 map[uint64]*replayASTMap
	fixedpoints          map[uint64]*replayFixedpoint
	optimizers           map[uint64]*replayOptimize
	rcfNumbers           map[uint64]*replayRCFNumber
	datatypeConstructors map[uint64]*replayDatatypeConstructor
	datatypeLists        map[uint64]*replayDatatypeList
	nextObjectID         uint64
	closed               bool
}

func NewZ3Replayer(input io.Reader, output io.Writer) (*Z3Replayer, error) {
	if input == nil || output == nil {
		return nil, ErrZ3ReplayState
	}
	return &Z3Replayer{scanner: newZ3ReplayScanner(bufio.NewReader(input)), output: output, heap: make(map[uint64]uint64), commands: make(map[uint32]replayCommandEntry)}, nil
}

func (r *Z3Replayer) Line() uint32 {
	if r == nil || r.scanner == nil {
		return 0
	}
	return r.scanner.line
}

func (r *Z3Replayer) Close() error {
	if r == nil || r.closed {
		return nil
	}
	for len(r.contextOrder) != 0 {
		i := len(r.contextOrder) - 1
		id := r.contextOrder[i]
		if context := r.contexts[id]; context != nil {
			if err := context.close(r); err != nil {
				return err
			}
			delete(r.contexts, id)
		}
		r.contextOrder = r.contextOrder[:i]
	}
	for len(r.configOrder) != 0 {
		i := len(r.configOrder) - 1
		id := r.configOrder[i]
		if config := r.configs[id]; config != nil {
			if err := config.Close(); err != nil {
				return err
			}
			delete(r.configs, id)
		}
		r.configOrder = r.configOrder[:i]
	}
	r.closed = true
	r.resetArguments()
	r.heap, r.commands, r.scanner, r.output, r.configs, r.contexts = nil, nil, nil, nil, nil, nil
	r.asts, r.astIDs = nil, nil
	r.parameterSets = nil
	r.descriptions = nil
	r.solvers = nil
	r.models = nil
	r.goals = nil
	r.vectors = nil
	r.functions = nil
	r.functionEntries = nil
	r.tactics = nil
	r.tacticResults = nil
	r.probes = nil
	r.statistics = nil
	r.maps = nil
	r.fixedpoints = nil
	r.optimizers = nil
	r.rcfNumbers = nil
	r.datatypeConstructors = nil
	r.datatypeLists = nil
	return nil
}
