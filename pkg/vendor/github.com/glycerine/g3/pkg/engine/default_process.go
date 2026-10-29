package engine

// Source: src/api/dll/mem_initializer.cpp and generated mem_initialize/finalize.

import (
	"errors"
	"fmt"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/process"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// defaultProcess owns the concrete Go services required by the source's
// first-call generated initializer. The retained port uses one goroutine.
type defaultProcess struct {
	budget   *arena.Budget
	account  *memory.ProcessAccounting
	runtime  *memory.Runtime
	rational *numeral.RationalGlobalService
	symbols  *symbol.Service
	region   *arena.BytesRegion
	debug    *diagnostic.DebugContext
	streams  *diagnostic.MemoryStandardStreams
	input    *diagnostic.NumericCharInput
	actions  *diagnostic.DebugActionContext
	ready    *process.Initialized
	complete bool
}

var apiDefaultProcess *defaultProcess

// InitializeDefaultProcess supplies the generated source owners on the first
// public API call. A previously initialized external process is used by the
// API directly and never becomes owned by this default process.
func InitializeDefaultProcess() error {
	if apiDefaultProcess != nil {
		return nil
	}
	if params.BorrowProcessGlobalParamsService() != nil {
		return nil
	}
	d := &defaultProcess{budget: arena.NewBudget(math.MaxUint64), account: memory.NewProcessAccounting()}
	fail := func(err error) error { return errors.Join(err, d.close()) }
	var err error
	d.runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: d.budget, Process: d.account})
	if err != nil {
		return fail(err)
	}
	d.rational, err = numeral.NewRationalGlobalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: d.runtime, Cells: arena.Config{PageSize: 2, Budget: d.budget},
		Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: d.budget}},
	}})
	if err != nil {
		return fail(err)
	}
	d.symbols, err = symbol.NewService(symbol.Config{Budget: d.budget})
	if err != nil {
		return fail(err)
	}
	d.region, err = arena.NewBytesRegion(arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: d.budget}})
	if err != nil {
		return fail(err)
	}
	d.debug, err = diagnostic.NewDebugContext(d.region, d.budget)
	if err != nil {
		return fail(err)
	}
	d.streams, err = diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: d.budget})
	if err != nil {
		return fail(err)
	}
	d.input, err = diagnostic.NewDebuggerCharInput(d.streams, nil)
	if err != nil {
		return fail(err)
	}
	d.actions, err = diagnostic.NewDebugActionContext(diagnostic.DebugActionConfig{
		Budget: d.budget, Streams: d.streams, Input: d.input, Profile: diagnostic.DebugInteractive,
	})
	if err != nil {
		return fail(err)
	}
	if err = d.debug.BindAssertionStreams(d.streams); err != nil {
		return fail(err)
	}
	if err = d.debug.BindAssertionActions(d.actions); err != nil {
		return fail(err)
	}
	cfg := process.InitializationConfig{Runtime: d.runtime, Rational: d.rational, Symbols: d.symbols, Debug: d.debug,
		Params:  params.ParamsConfig{Budget: d.budget, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: d.budget}}},
		Catalog: params.GlobalParamsCatalogConfig{Budget: d.budget, Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: d.budget}}, Text: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: d.budget}}},
		Globals: params.GlobalParamsConfig{Budget: d.budget, DescriptorPageSize: 2, DescriptorWordsPageSize: 16}}
	d.ready, err = InitializeGenerated(cfg, math.MaxUint32)
	if err != nil {
		return fail(err)
	}
	d.complete = true
	apiDefaultProcess = d
	return nil
}

// FinalizeDefaultProcess releases only the process constructed by
// InitializeDefaultProcess. An external generated process is caller-owned.
func FinalizeDefaultProcess() error {
	if apiDefaultProcess == nil {
		return nil
	}
	if apiDefaultProcess.ready != nil && params.BorrowProcessGlobalParamsService() != apiDefaultProcess.ready.Globals {
		return arena.ErrConfig
	}
	if err := apiDefaultProcess.close(); err != nil {
		return err
	}
	apiDefaultProcess = nil
	return nil
}

func (d *defaultProcess) close() error {
	if d == nil {
		return nil
	}
	if d.complete {
		if err := process.FinalizeGenerated(d.account, d.debug, d.ready.Globals, d.rational, d.symbols, d.ready.Trace); err != nil {
			return err
		}
		d.complete = false
	}
	if d.ready != nil {
		if err := d.ready.Close(); err != nil {
			return err
		}
		d.ready = nil
	}
	if d.rational != nil {
		if err := d.rational.Close(); err != nil {
			return err
		}
		d.rational = nil
	}
	if d.symbols != nil {
		if err := d.symbols.Close(); err != nil {
			return err
		}
		d.symbols = nil
	}
	if d.debug != nil {
		if err := d.debug.Close(); err != nil {
			return err
		}
		d.debug = nil
	}
	if d.actions != nil {
		if err := d.actions.Close(); err != nil {
			return err
		}
		d.actions = nil
	}
	if d.input != nil {
		if err := d.input.Close(); err != nil {
			return err
		}
		d.input = nil
	}
	if d.streams != nil {
		if err := d.streams.Close(); err != nil {
			return err
		}
		d.streams = nil
	}
	if d.region != nil {
		if err := d.region.Close(); err != nil {
			return err
		}
		d.region = nil
	}
	if d.runtime != nil {
		if err := d.runtime.Close(); err != nil {
			return err
		}
		d.runtime = nil
	}
	if used := d.budget.Used(); used != 0 {
		return fmt.Errorf("engine: default process retained %d charged bytes", used)
	}
	return nil
}
