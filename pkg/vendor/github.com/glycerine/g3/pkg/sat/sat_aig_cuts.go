package sat

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type satAIGBoolOp uint8

const (
	satAIGVar satAIGBoolOp = iota
	satAIGAnd
	satAIGITE
	satAIGXor
	satAIGLUT
	satAIGNone
)

type satAIGCutsConfig struct {
	maxCutSetSize, maxAux, maxInsertions uint32
	full                                 bool
}

func defaultSATAIGCutsConfig() satAIGCutsConfig {
	return satAIGCutsConfig{maxCutSetSize: 20, maxAux: 5, maxInsertions: 20, full: true}
}

type satAIGClauseUpdate func([]Literal)

type satAIGCuts struct {
	self                             *satAIGCuts
	budget                           *arena.Budget
	ownerBytes                       uint64
	closed                           bool
	random                           *z3rand.Generator
	config                           satAIGCutsConfig
	nodes                            [][]satAIGNode
	literals                         []Literal
	cuts                             []*satCutSet
	maxCutSetSize, lastTouched       []uint32
	numCutCalls, numCuts, insertions uint32
	roots                            []satAIGRoot
	onClauseAdd, onClauseDel         satAIGClauseUpdate
	onCutAdd, onCutDel               satCutUpdate
}

func newSATAIGCuts(b *arena.Budget) (*satAIGCuts, error) {
	n := uint64(reflect.TypeOf(satAIGCuts{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	a := &satAIGCuts{budget: b, ownerBytes: n, random: z3rand.New(0), config: defaultSATAIGCutsConfig()}
	a.self = a
	return a, nil
}

func (a *satAIGCuts) open() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed {
		return arena.ErrClosed
	}
	return nil
}

func (a *satAIGCuts) NumCuts() uint32 {
	if a == nil || a.closed {
		return 0
	}
	return a.numCuts
}
func (a *satAIGCuts) MaxCutSetSize(v uint32) uint32 {
	if v == math.MaxUint32 || int(v) >= len(a.maxCutSetSize) {
		return a.config.maxCutSetSize
	}
	return a.maxCutSetSize[v]
}
