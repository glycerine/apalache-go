package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:82-118.

import (
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/params"
)

// EngineKind follows the source's five concrete numeral choices. The selected
// tactic adapter rebuilds the concrete branch when the source numeral option changes.
type EngineKind uint8

const (
	EngineMPQ EngineKind = iota
	EngineMPF
	EngineHWF
	EngineMPFF
	EngineMPFX
	EngineNone
)

type Options struct {
	PrintNodes bool
	Engine     EngineKind
}

// ReadOptions preserves the tactic's print_nodes-then-numeral lookup order.
// An unrecognized numeral name selects HWF, exactly as the source switch does.
func ReadOptions(pc *params.ParamsContext, p *params.ParamsRef) (result Options, err error) {
	result.Engine = EngineNone
	if pc == nil || p == nil {
		return result, params.ErrParamsOwner
	}
	printNodes := "print_nodes"
	result.PrintNodes, err = pc.GetBoolName(p, &printNodes, false)
	if err != nil {
		return result, err
	}
	defaultName, err := pc.SymbolsField.Intern("mpq")
	if err != nil {
		return result, err
	}
	engineName := "numeral"
	engine, err := pc.GetSymbolName(p, &engineName, defaultName)
	if err != nil {
		return result, err
	}
	for _, candidate := range []struct {
		name string
		kind EngineKind
	}{
		{"mpq", EngineMPQ},
		{"mpf", EngineMPF},
		{"mpff", EngineMPFF},
		{"mpfx", EngineMPFX},
	} {
		match, e := pc.SymbolsField.EqualString(engine, &candidate.name)
		if e != nil {
			return result, e
		}
		if match {
			result.Engine = candidate.kind
			return result, nil
		}
	}
	result.Engine = EngineHWF
	return result, nil
}

// CollectParamDescriptions appends the context's five unsigned options before
// the tactic's numeral and print_nodes options, matching the source order.
func CollectParamDescriptions(d *params.ParamDescrs) error {
	if err := subpaving.CollectHardwareParamDescrs(d); err != nil {
		return err
	}
	for _, entry := range []struct {
		name, description string
		kind              params.ParamKind
	}{
		{"numeral", "(default: mpq) options: mpq, mpf, hwf, mpff, mpfx.", params.ParamSymbol},
		{"print_nodes", "(default: false) display subpaving tree leaves.", params.ParamBool},
	} {
		if err := d.InsertName(entry.name, entry.kind, &entry.description, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
