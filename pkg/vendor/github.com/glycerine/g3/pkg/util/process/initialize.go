package process

// Source: src/api/dll/mem_initializer.cpp, src/shell/mem_initializer.cpp,
// src/test/mem_initializer.cpp, generated mem_initialize body.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var ErrProcessGlobalParamsBinding = errors.New("process: generated global parameter binding conflict")

// InitializationConfig supplies the two source static services and the Go
// storage configurations needed to construct gparams after symbols exist.
// The runtime and services are borrowed by the generated binding. A nil Trace
// uses the process trace tags; pass the same explicit owner to finalization.
type InitializationConfig struct {
	Runtime  *memory.Runtime
	Rational *numeral.RationalGlobalService
	Symbols  *symbol.Service
	Debug    *diagnostic.DebugContext
	Trace    *diagnostic.TraceTags
	Params   params.ParamsConfig
	Catalog  params.GlobalParamsCatalogConfig
	Globals  params.GlobalParamsConfig
}

// Initialized owns only the Go parameter context/catalog/service prepared
// during generated initialization. Rational, symbol, memory and trace owners
// remain with the caller. A partial result is returned with any stage error so
// its completed Go owners can still be closed.
type Initialized struct {
	self               *Initialized
	process            *memory.ProcessAccounting
	debug              *diagnostic.DebugContext
	rational           *numeral.RationalGlobalService
	symbols            *symbol.Service
	Context            *params.ParamsContext
	Catalog            *params.GlobalParamsCatalog
	Globals            *params.GlobalParamsService
	Trace              *diagnostic.TraceTags
	debugPrevious      *diagnostic.DebugContext
	debugTracePrevious *diagnostic.TraceTags
	debugTraceBound    bool
	paramsPrevious     *params.GlobalParamsService
	paramsBound        bool
	closed             bool
}

// Generated mem_initialize/mem_finalize use one fixed process cohort. Keep
// the exact borrowed children for finalization argument validation.
var currentGenerated *Initialized

// InitializeGenerated executes the first-call source order. A repeated call
// updates the shared maximum through memory::initialize and returns nil,nil;
// the caller retains the first result for later finalization and owning close.
func InitializeGenerated(c InitializationConfig, maxSize uint64) (*Initialized, error) {
	if c.Runtime == nil {
		return nil, arena.ErrConfig
	}
	p := c.Runtime.ProcessOwner()
	if p == nil {
		return nil, arena.ErrConfig
	}
	// Source repeated calls update the maximum and return without reading any
	// generated child. This also preserves the pending-stage guard in Runtime.
	if p.InitializationStarted() {
		_, err := c.Runtime.BeginSourceInitialize(maxSize)
		return nil, err
	}
	// C++ has one process g_imp. A second generated process must not replace a
	// live binding from a different memory process.
	if params.BorrowProcessGlobalParamsService() != nil {
		return nil, ErrProcessGlobalParamsBinding
	}
	if c.Rational == nil || c.Symbols == nil || c.Debug == nil || c.Params.Budget == nil ||
		c.Catalog.Budget != c.Params.Budget || c.Globals.Budget != c.Params.Budget || c.Globals.Catalog != nil {
		return nil, arena.ErrConfig
	}
	if _, err := c.Debug.AssertionsEnabled(); err != nil {
		return nil, err
	}
	rationalProcess, err := c.Rational.ProcessOwner()
	if err != nil {
		return nil, err
	}
	if rationalProcess == nil || rationalProcess != c.Runtime.ProcessOwner() {
		return nil, arena.ErrConfig
	}
	first, err := c.Runtime.BeginSourceInitialize(maxSize)
	if err != nil || !first {
		return nil, err
	}
	result := &Initialized{process: p, debug: c.Debug, rational: c.Rational, symbols: c.Symbols}
	result.self = result
	if err = c.Rational.BindMemoryTrace(c.Trace); err != nil {
		return result, err
	}
	if err = c.Rational.InitializeSource(); err != nil {
		return result, err
	}
	if err = c.Symbols.BindMemoryProcess(p); err != nil {
		return result, err
	}
	if err = c.Symbols.BindTrace(c.Trace); err != nil {
		return result, err
	}
	if err = c.Symbols.InitializeSource(); err != nil {
		return result, err
	}
	s, err := c.Symbols.Manager()
	if err != nil {
		return result, err
	}
	if err = s.BindDebug(c.Debug); err != nil {
		return result, err
	}
	q, err := c.Rational.Service()
	if err != nil {
		return result, err
	}
	result.Context, err = params.NewParamsContext(s, q, c.Params)
	if err != nil {
		return result, err
	}
	if err = result.Context.BindMemoryTrace(c.Trace); err != nil {
		return result, err
	}
	if err = result.Context.BindMemoryProcess(p); err != nil {
		return result, err
	}
	result.Catalog, err = params.NewGlobalParamsCatalog(s, c.Catalog)
	if err != nil {
		return result, err
	}
	c.Globals.Catalog = result.Catalog
	result.Globals, err = params.NewGlobalParamsService(s, result.Context, c.Globals)
	if err != nil {
		return result, err
	}
	if err = result.Globals.BindNamespaceAssertions(c.Debug); err != nil {
		return result, err
	}
	if err = result.Globals.BindMemoryProcess(c.Runtime.ProcessOwner()); err != nil {
		return result, err
	}
	result.Trace = c.Trace
	if result.Trace == nil {
		result.Trace = diagnostic.BorrowProcessTraceTags()
	}
	if err = result.Trace.BindMemoryProcess(p); err != nil {
		return result, err
	}
	result.debugTracePrevious, err = c.Debug.BindMemoryTrace(result.Trace)
	if err != nil {
		return result, err
	}
	result.debugTraceBound = true
	if err = result.Globals.BindNamespaceTrace(result.Trace); err != nil {
		return result, err
	}
	if err = result.Globals.InitializeSource(); err != nil {
		return result, err
	}
	result.paramsPrevious, err = params.BindProcessGlobalParamsService(result.Globals)
	if err != nil {
		return result, err
	}
	result.paramsBound = true
	if err = c.Runtime.CompleteSourceInitialize(); err != nil {
		return result, err
	}
	result.debugPrevious, err = diagnostic.BindProcessDebugContext(c.Debug)
	if err != nil {
		return result, err
	}
	currentGenerated = result
	return result, nil
}

// Close releases only the Go owners prepared by InitializeGenerated, after
// the caller has finished using them and any generated finalization has run.
func (v *Initialized) Close() error {
	if v == nil || v.self != v {
		return arena.ErrConfig
	}
	if v.closed {
		return nil
	}
	if v.paramsBound && v.Globals != nil && params.BorrowProcessGlobalParamsService() != v.Globals {
		return ErrProcessGlobalParamsBinding
	}
	// A stopped source finalizer retains its finalizing guard; owning Close may
	// still drain that child. A live process must run generated finalization first.
	if currentGenerated == v && v.process.InitializationStarted() && !v.process.Finalizing() {
		return arena.ErrConfig
	}
	if v.Globals != nil {
		if err := v.Globals.Close(); err != nil {
			return err
		}
		v.Globals = nil
	}
	if v.paramsBound {
		if _, err := params.BindProcessGlobalParamsService(v.paramsPrevious); err != nil {
			return err
		}
		v.paramsPrevious = nil
		v.paramsBound = false
	}
	if v.Catalog != nil {
		if err := v.Catalog.Close(); err != nil {
			return err
		}
		v.Catalog = nil
	}
	if v.Context != nil {
		if err := v.Context.Close(); err != nil {
			return err
		}
		v.Context = nil
	}
	if v.debugTraceBound {
		if _, err := v.debug.BindMemoryTrace(v.debugTracePrevious); err != nil {
			return err
		}
		v.debugTracePrevious = nil
		v.debugTraceBound = false
	}
	if v.Trace != nil {
		if err := v.Trace.BindMemoryProcess(nil); err != nil {
			return err
		}
		v.Trace = nil
	}
	if v.debugPrevious != nil {
		if _, err := diagnostic.BindProcessDebugContext(v.debugPrevious); err != nil {
			return err
		}
		v.debugPrevious = nil
	}
	if currentGenerated == v {
		currentGenerated = nil
	}
	v.process, v.debug, v.rational, v.symbols = nil, nil, nil, nil
	v.closed = true
	return nil
}
