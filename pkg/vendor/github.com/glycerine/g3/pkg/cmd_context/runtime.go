package cmd_context

// Source: src/cmd_context/cmd_context.cpp init_manager and reset ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type contextServices struct {
	budget      *arena.Budget
	symbols     *usymbol.Manager
	numerals    *unumeral.RationalService
	params      *uparams.ParamsContext
	catalog     *uparams.GlobalParamsCatalog
	globals     *uparams.GlobalParamsService
	cooperation *cooperate.Cooperation
	memory      *umemory.Runtime
	fatal       *diagnostic.MemoryFatalContext
	streams     *diagnostic.MemoryStandardStreams
}

// NewStandaloneContext owns the services required by a command session. The
// executable composition root must link the generated collectors in engine,
// matching the source shell's process initialization before cmd_context.
func NewStandaloneContext() (*Context, error) {
	context, _, err := newStandaloneContext(false)
	return context, err
}

// NewStandaloneShellContext constructs the command-line process memory path.
// The returned policy is borrowed from the context and is closed with it.
func NewStandaloneShellContext() (*Context, *diagnostic.MemoryFatalContext, error) {
	return newStandaloneContext(true)
}

func newStandaloneContext(shellMemory bool) (*Context, *diagnostic.MemoryFatalContext, error) {
	s := &contextServices{budget: arena.NewBudget(1 << 40)}
	fail := func(err error) (*Context, *diagnostic.MemoryFatalContext, error) {
		return nil, nil, errors.Join(err, s.Close())
	}
	var err error
	if shellMemory {
		s.streams, err = diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: s.budget})
		if err != nil {
			return fail(err)
		}
		s.fatal, err = diagnostic.NewMemoryFatalContext(diagnostic.MemoryFatalContextConfig{Budget: s.budget, Streams: s.streams})
		if err != nil {
			return fail(err)
		}
		s.memory, err = umemory.NewRuntime(umemory.RuntimeConfig{Budget: s.budget, FatalPolicy: s.fatal})
		if err != nil {
			return fail(err)
		}
	}
	s.symbols, err = usymbol.NewManager(usymbol.Config{Budget: s.budget})
	if err != nil {
		return fail(err)
	}
	integers := unumeral.IntegerConfig{Cells: arena.Config{Budget: s.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: s.budget}}, Memory: s.memory}
	s.numerals, err = unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: integers})
	if err != nil {
		return fail(err)
	}
	s.params, err = uparams.NewParamsContext(s.symbols, s.numerals, uparams.ParamsConfig{Budget: s.budget})
	if err != nil {
		return fail(err)
	}
	s.catalog, err = uparams.NewGlobalParamsCatalog(s.symbols, uparams.GlobalParamsCatalogConfig{Budget: s.budget})
	if err != nil {
		return fail(err)
	}
	s.globals, err = uparams.NewGlobalParamsService(s.symbols, s.params, uparams.GlobalParamsConfig{Budget: s.budget, Catalog: s.catalog})
	if err != nil {
		return fail(err)
	}
	if err = s.globals.InitializeSource(); err != nil {
		return fail(err)
	}
	s.cooperation, err = cooperate.NewCooperation(s.budget)
	if err != nil {
		return fail(err)
	}
	config := smtcore.SMTSolverConfig{Params: s.params, Globals: s.globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: s.params, Globals: s.globals, Memory: s.numerals.RuntimeOwner(), Cooperation: s.cooperation}}
	c, err := NewContext(config)
	if c != nil {
		c.owned = s
	}
	if err != nil {
		if c != nil {
			return nil, nil, errors.Join(err, c.Close())
		}
		return fail(err)
	}
	return c, s.fatal, nil
}

// NewAuxiliaryContext constructs the non-main command context used by API
// parsers, which must not redirect the process diagnostic streams.
func NewAuxiliaryContext() (*Context, error) {
	c, err := NewStandaloneContext()
	if c != nil {
		c.mainContext = false
	}
	return c, err
}

func (s *contextServices) Close() (result error) {
	if s.cooperation != nil {
		result = errors.Join(result, s.cooperation.Close())
		s.cooperation = nil
	}
	if s.globals != nil {
		result = errors.Join(result, s.globals.Close())
		s.globals = nil
	}
	if s.catalog != nil {
		result = errors.Join(result, s.catalog.Close())
		s.catalog = nil
	}
	if s.params != nil {
		result = errors.Join(result, s.params.Close())
		s.params = nil
	}
	if s.numerals != nil {
		result = errors.Join(result, s.numerals.Close())
		s.numerals = nil
	}
	if s.memory != nil {
		result = errors.Join(result, s.memory.Close())
		s.memory = nil
	}
	if s.fatal != nil {
		result = errors.Join(result, s.fatal.Close())
		s.fatal = nil
	}
	if s.streams != nil {
		result = errors.Join(result, s.streams.Close())
		s.streams = nil
	}
	if s.symbols != nil {
		result = errors.Join(result, s.symbols.Close())
		s.symbols = nil
	}
	return result
}
