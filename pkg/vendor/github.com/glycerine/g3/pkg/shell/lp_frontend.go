package shell

// Source: src/shell/lp_frontend.cpp:56-107; lp_frontend.h.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/control"
	lp "github.com/glycerine/g3/pkg/util/lp"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type LPFrontendConfig struct {
	Globals *uparams.GlobalParamsService
	Input   io.Reader
	Output  io.Writer
	Name    string
}

type LPFrontend struct {
	globals *uparams.GlobalParamsService
	params  *lp.LPParams
	limit   *control.Reslimit
	timer   *control.ScopedTimer
	reader  *lp.FloatMPSReader
	solver  *lp.FloatPrimalSimplex
	input   io.Reader
	output  io.Writer
	name    string
	min     bool
	running bool
	closed  bool
}

func NewLPFrontend(c LPFrontendConfig) (f *LPFrontend, err error) {
	if c.Globals == nil || c.Input == nil || c.Output == nil {
		return nil, ErrShellOptionService
	}
	f = &LPFrontend{globals: c.Globals, input: c.Input, output: c.Output, name: c.Name}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
			f = nil
		}
	}()
	f.params, err = lp.NewLPParams(c.Globals.Context, nil, c.Globals, c.Globals.Context.Budget())
	if err != nil {
		return nil, err
	}
	f.limit, err = control.NewReslimit(c.Globals.Context.Budget())
	return f, err
}

func (f *LPFrontend) Run() (err error) {
	if f == nil || f.closed || f.running {
		return ErrShellOptionService
	}
	f.running = true
	defer func() { f.running = false }()
	if err = f.startLimits(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.stopLimits()) }()
	f.reader = lp.NewFloatMPSReader(f.input)
	if err = f.reader.SetMessageStream(f.output); err != nil {
		return err
	}
	if err = f.reader.Read(); err != nil {
		return f.cannotProcess(err)
	}
	if !f.reader.IsOK() {
		return f.cannotProcess(nil)
	}
	f.solver, err = f.reader.CreatePrimalSolver()
	if err != nil {
		return err
	}
	if err = f.configureSolver(); err != nil {
		return err
	}
	if err = f.solver.FindMaximalSolution(); err != nil {
		return err
	}
	return f.writeLPResult()
}
