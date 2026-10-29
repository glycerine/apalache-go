package params

// Source: src/util/env_params.cpp:25-33; src/util/util.h:391-399.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// EnvParamsRuntime binds the source process-global update to its concrete Go
// owners. It borrows all four owners and retains no independent state.
type EnvParamsRuntime struct {
	globals  *GlobalParams
	verbose  *diagnostic.VerboseContext
	memory   *memory.Runtime
	warnings *diagnostic.WarningService
}

func NewEnvParamsRuntime(g *GlobalParams, v *diagnostic.VerboseContext, m *memory.Runtime, w *diagnostic.WarningService) (*EnvParamsRuntime, error) {
	if g == nil || g.ContextField == nil || v == nil || m == nil || w == nil {
		return nil, arena.ErrConfig
	}
	q := g.ContextField.Numerals()
	if q == nil || q.RuntimeOwner() == nil {
		return nil, arena.ErrConfig
	}
	process, numericProcess := m.ProcessOwner(), q.RuntimeOwner().ProcessOwner()
	// Separate standalone runtimes have separate counters even though both
	// report no process owner. The source updates one memory namespace.
	if process != numericProcess || (process == nil && m != q.RuntimeOwner()) {
		return nil, arena.ErrConfig
	}
	return &EnvParamsRuntime{globals: g, verbose: v, memory: m, warnings: w}, nil
}

// UpdateProcessEnvParams binds env_params::updt_params to a current concrete
// gparams owner and the retained process-wide verbosity and warning owners.
// The rational service's runtime supplies the shared memory process limits.
func UpdateProcessEnvParams(g *GlobalParams) error {
	if g == nil || g.ContextField == nil {
		return arena.ErrConfig
	}
	q := g.ContextField.Numerals()
	if q == nil || q.RuntimeOwner() == nil {
		return arena.ErrConfig
	}
	env, err := NewEnvParamsRuntime(g, diagnostic.BorrowProcessVerboseContext(), q.RuntimeOwner(), diagnostic.BorrowProcessWarningService())
	if err != nil {
		return err
	}
	return env.Update()
}

func (e *EnvParamsRuntime) WarningsEnabled() bool { return e != nil && e.warnings.Enabled() }

// Update follows the five source statements. A later failed getter or setter
// leaves earlier assignments visible, as env_params::updt_params does.
func (e *EnvParamsRuntime) Update() error {
	if e == nil || e.globals == nil || e.verbose == nil || e.memory == nil || e.warnings == nil {
		return arena.ErrConfig
	}
	p, err := e.globals.GetRef()
	if err != nil {
		return err
	}
	defaultVerbosity, err := e.verbose.VerbosityLevel()
	if err != nil {
		return err
	}
	verbose := "verbose"
	level, err := e.globals.ContextField.GetUIntName(p, &verbose, defaultVerbosity)
	if err != nil {
		return err
	}
	if err = e.verbose.SetVerbosityLevel(level); err != nil {
		return err
	}
	warning := "warning"
	enabled, err := e.globals.ContextField.GetBoolName(p, &warning, true)
	if err != nil {
		return err
	}
	if err = e.warnings.Enable(enabled); err != nil {
		return err
	}
	maxSize := "memory_max_size"
	megabytes, err := e.globals.ContextField.GetUIntName(p, &maxSize, 0)
	if err != nil {
		return err
	}
	if err = e.memory.SetMaxSizeMegabytes(megabytes); err != nil {
		return err
	}
	maxCount := "memory_max_alloc_count"
	count, err := e.globals.ContextField.GetUIntName(p, &maxCount, 0)
	if err != nil {
		return err
	}
	if err = e.memory.SetMaxCount(uint64(count)); err != nil {
		return err
	}
	watermark := "memory_high_watermark"
	mark, err := e.globals.ContextField.GetUIntName(p, &watermark, 0)
	if err != nil {
		return err
	}
	return e.memory.SetWatermark(uint64(mark))
}
