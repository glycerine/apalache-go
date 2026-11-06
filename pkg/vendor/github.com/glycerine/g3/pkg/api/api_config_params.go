package api

// Source: src/api/api_config_params.cpp:69-94.

import (
	"errors"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var ErrConfigOwner = errors.New("api: invalid or closed config")

// Config owns the concrete context_params object returned by Z3_mk_config.
type Config struct {
	params  *smtcore.ContextParams
	trace   *diagnostic.TraceTags
	process *memory.ProcessAccounting
}

// MkConfig initializes the process and constructs context_params from its
// current global parameter state.
func MkConfig() (*Config, error) {
	g, err := globalParamService()
	if err != nil {
		return nil, err
	}
	var log *Z3LogWriter
	if processZ3Log != nil && !processZ3LogInAPI {
		processZ3LogInAPI = true
		defer func() { processZ3LogInAPI = false }()
		log = processZ3Log
		_ = log.LogZ3MkConfig()
	}
	config, err := mkConfigWithGlobals(g, diagnostic.BorrowProcessTraceTags())
	if err != nil {
		return nil, err
	}
	if log != nil {
		if id, e := log.configID(config); e == nil {
			_ = log.Result(id)
		}
	}
	return config, nil
}

func mkConfigWithTrace(tags *diagnostic.TraceTags) (*Config, error) {
	g, err := globalParamService()
	if err != nil {
		return nil, err
	}
	return mkConfigWithGlobals(g, tags)
}

func mkConfigWithGlobals(g *uparams.GlobalParamsService, tags *diagnostic.TraceTags) (*Config, error) {
	p, err := smtcore.NewContextParamsWithAllocationHook(g.Context, g, g.Config.Budget, func(p *smtcore.ContextParams) {
		traceConfigAllocated(tags, p)
	})
	if err != nil {
		if p != nil {
			return nil, errors.Join(err, p.Close())
		}
		return nil, err
	}
	return &Config{params: p, trace: tags, process: g.Context.Numerals().RuntimeOwner().ProcessOwner()}, nil
}

// Close is the owning Z3_del_config operation.
func (c *Config) Close() error {
	if c == nil || c.params == nil {
		return ErrConfigOwner
	}
	var log *Z3LogWriter
	if processZ3Log != nil && !processZ3LogInAPI {
		processZ3LogInAPI = true
		defer func() { processZ3LogInAPI = false }()
		log = processZ3Log
		if id, err := log.configID(c); err == nil {
			_ = log.LogZ3DelConfig(id)
		}
	}
	p := c.params
	if err := p.Close(); err != nil {
		return err
	}
	c.traceConfigDeallocated(p)
	c.params = nil
	if log != nil {
		delete(log.configIDs, c)
	}
	return nil
}

// SetParamValue retains Z3_set_param_value's catch: a project exception is
// reported through the process warning stream and does not escape this call.
func SetParamValue(c *Config, id, value string) error {
	if c == nil || c.params == nil {
		return ErrConfigOwner
	}
	if processZ3Log != nil && !processZ3LogInAPI {
		processZ3LogInAPI = true
		defer func() { processZ3LogInAPI = false }()
		if pointer, err := processZ3Log.configID(c); err == nil {
			_ = processZ3Log.LogZ3SetParamValue(pointer, id, value)
		}
	}
	return globalParamCaughtWarning(c.params.Set(&id, &value))
}

// UpdateParamValue updates this context's construction parameters from their
// textual API representation, matching Z3_update_param_value.
func (c *SMTContext) UpdateParamValue(id, value string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.UpdateParamValue(id, value)
}
