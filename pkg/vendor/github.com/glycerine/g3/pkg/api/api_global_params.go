package api

// Source: src/api/api_config_params.cpp:30-69.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrGlobalParamProcessUninitialized = errors.New("api: generated global parameter process is not initialized")

// globalParamService follows memory::initialize(UINT_MAX) on every API call.
// The first call constructs the generated process through engine; subsequent
// calls borrow either that process or a caller-supplied generated process.
func globalParamService() (*params.GlobalParamsService, error) {
	g := params.BorrowProcessGlobalParamsService()
	if g == nil {
		if err := engine.InitializeDefaultProcess(); err != nil {
			return nil, err
		}
		g = params.BorrowProcessGlobalParamsService()
	}
	if g == nil || g.Context == nil {
		return nil, ErrGlobalParamProcessUninitialized
	}
	q := g.Context.Numerals()
	if q == nil || q.RuntimeOwner() == nil {
		return nil, ErrGlobalParamProcessUninitialized
	}
	runtime := q.RuntimeOwner()
	process := runtime.ProcessOwner()
	if process == nil || !process.InitializationStarted() {
		return nil, ErrGlobalParamProcessUninitialized
	}
	first, err := runtime.BeginSourceInitialize(math.MaxUint32)
	if err != nil {
		return nil, err
	}
	if first {
		return nil, ErrGlobalParamProcessUninitialized
	}
	return g, nil
}

// GlobalParamSet applies the source global setter and then refreshes the
// process environment. The C API reports z3_exception through warning_msg
// because this entry point has no context error handler.
func GlobalParamSet(id, value string) error {
	g, err := globalParamService()
	if err != nil {
		return err
	}
	if err = g.SourceSetName(id, value); err == nil {
		manager, e := g.Manager()
		if e != nil {
			err = e
		} else {
			err = params.UpdateProcessEnvParams(manager)
		}
	}
	if err != nil {
		return globalParamCaughtWarning(err)
	}
	return nil
}

func globalParamCaughtWarning(err error) error {
	var exception z3exception.Z3Exception
	if !errors.As(err, &exception) {
		return err
	}
	return diagnostic.BorrowProcessWarningService().Warningf("%s", exception.Message())
}

// GlobalParamResetAll has no source catch: reset and the environment update
// stop at the first error.
func GlobalParamResetAll() error {
	g, err := globalParamService()
	if err != nil {
		return err
	}
	if err = g.SourceReset(); err != nil {
		return err
	}
	manager, err := g.Manager()
	if err != nil {
		return err
	}
	return params.UpdateProcessEnvParams(manager)
}

// GlobalParamGet owns its Go string. A failed source lookup returns false
// after warning_msg, matching Z3_global_param_get's null output and false.
func GlobalParamGet(id string) (value string, ok bool, err error) {
	g, err := globalParamService()
	if err != nil {
		return "", false, err
	}
	value, err = g.SourceGetValueName(id)
	if err != nil {
		if warning := globalParamCaughtWarning(err); warning != nil {
			return "", false, warning
		}
		return "", false, nil
	}
	return value, true, nil
}
