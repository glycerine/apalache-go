package opt

// Source: src/opt/opt_params.hpp; src/opt/opt_params.pyg.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Params is the generated optimization parameter facade. It borrows the local
// value and owns the constructor-time global opt-module snapshot.
type Params struct {
	self              *Params
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewParams(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef) (*Params, error) {
	if context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	p := &Params{context: context}
	p.self = p
	if local == nil {
		p.empty = context.EmptyValue()
		local = &p.empty
	}
	if err := context.Check(local); err != nil {
		return nil, err
	}
	p.local = local
	var err error
	p.global, err = globals.SourceGetModuleName("opt")
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Params) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *Params) symbol(key, def string) (usymbol.Symbol, error) {
	if err := p.open(); err != nil {
		return usymbol.Symbol{}, err
	}
	d, err := p.context.SymbolsField.Intern(def)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	return p.context.GetSymbolFallbackName(p.local, &key, &p.global, d)
}

func (p *Params) boolean(key string, def bool) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, def)
}

func (p *Params) unsigned(key string, def uint32) (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, def)
}

func (p *Params) OptSMTEngine() (usymbol.Symbol, error) {
	return p.symbol("optsmt_engine", "basic")
}
func (p *Params) MaxSATEngine() (usymbol.Symbol, error) {
	return p.symbol("maxsat_engine", "maxres")
}
func (p *Params) Priority() (usymbol.Symbol, error) { return p.symbol("priority", "lex") }
func (p *Params) DumpBenchmarks() (bool, error) {
	return p.boolean("dump_benchmarks", false)
}
func (p *Params) SolutionPrefix() (usymbol.Symbol, error) {
	return p.symbol("solution_prefix", "")
}
func (p *Params) Timeout() (uint32, error) { return p.unsigned("timeout", ^uint32(0)) }
func (p *Params) RLimit() (uint32, error)  { return p.unsigned("rlimit", 0) }
func (p *Params) EnableSLS() (bool, error) { return p.boolean("enable_sls", false) }
func (p *Params) EnableSAT() (bool, error) { return p.boolean("enable_sat", true) }
func (p *Params) Elim01() (bool, error)    { return p.boolean("elim_01", true) }
func (p *Params) PPNeat() (bool, error)    { return p.boolean("pp.neat", true) }
func (p *Params) PBCompileEquality() (bool, error) {
	return p.boolean("pb.compile_equality", false)
}
func (p *Params) MaxResHillClimb() (bool, error) {
	return p.boolean("maxres.hill_climb", true)
}
func (p *Params) MaxResAddUpperBoundBlock() (bool, error) {
	return p.boolean("maxres.add_upper_bound_block", false)
}
func (p *Params) MaxResMaxNumCores() (uint32, error) {
	return p.unsigned("maxres.max_num_cores", ^uint32(0))
}
func (p *Params) MaxResMaxCoreSize() (uint32, error) {
	return p.unsigned("maxres.max_core_size", 3)
}
func (p *Params) MaxResMaximizeAssignment() (bool, error) {
	return p.boolean("maxres.maximize_assignment", false)
}
func (p *Params) MaxResMaxCorrectionSetSize() (uint32, error) {
	return p.unsigned("maxres.max_correction_set_size", 3)
}
func (p *Params) MaxResWMax() (bool, error) {
	return p.boolean("maxres.wmax", false)
}
func (p *Params) MaxResPivotOnCorrectionSet() (bool, error) {
	return p.boolean("maxres.pivot_on_correction_set", true)
}

func (p *Params) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	if p.destructorStarted {
		if err := p.context.CompleteDecrement(&p.global, p.decrement); err != nil {
			return err
		}
	} else {
		p.destructorStarted = true
		if err := p.context.DelTracked(&p.global, &p.decrement); err != nil {
			return err
		}
	}
	p.closed = true
	return nil
}
