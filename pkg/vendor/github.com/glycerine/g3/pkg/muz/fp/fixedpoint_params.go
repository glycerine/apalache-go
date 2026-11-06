package fp

// Source: src/muz/fp/dl_cmds.cpp query-local params_ref update and
// src/muz/base/fixedpoint_params.hpp local-over-module lookup precedence.

import (
	"errors"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fixedpointParamView struct {
	context *uparams.ParamsContext
	local   *uparams.ParamsRef
	global  uparams.ParamsRef
	receipt uparams.ParamsDecrementReceipt
}

func (e *PropositionalEngine) fixedpointParamView() (*fixedpointParamView, error) {
	if e.params == nil || e.globals == nil {
		return nil, ErrPropositionalEngine
	}
	global, err := e.globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return nil, err
	}
	view := &fixedpointParamView{context: e.params, global: global}
	if e.queryParamsReady {
		view.local = &e.queryParams
	}
	return view, nil
}

func (v *fixedpointParamView) closeOwned() error {
	if v == nil || v.context == nil {
		return nil
	}
	if v.receipt.Invocation() != 0 {
		if err := v.context.CompleteDecrement(&v.global, v.receipt); err != nil {
			return err
		}
		v.receipt = uparams.ParamsDecrementReceipt{}
		v.context, v.local = nil, nil
		return nil
	}
	var receipt uparams.ParamsDecrementReceipt
	if err := v.context.DelTracked(&v.global, &receipt); err != nil {
		if receipt.Invocation() != 0 {
			v.receipt = receipt
		}
		return err
	}
	v.context, v.local = nil, nil
	return nil
}

func (v *fixedpointParamView) Bool(name string, fallback bool) (bool, error) {
	if v.local == nil {
		return v.context.GetBoolName(&v.global, &name, fallback)
	}
	return v.context.GetBoolFallbackName(v.local, &name, &v.global, fallback)
}

func (v *fixedpointParamView) UInt(name string, fallback uint32) (uint32, error) {
	if v.local == nil {
		return v.context.GetUIntName(&v.global, &name, fallback)
	}
	return v.context.GetUIntFallbackName(v.local, &name, &v.global, fallback)
}

func (v *fixedpointParamView) Symbol(name string, fallback usymbol.Symbol) (usymbol.Symbol, error) {
	if v.local == nil {
		return v.context.GetSymbolName(&v.global, &name, fallback)
	}
	return v.context.GetSymbolFallbackName(v.local, &name, &v.global, fallback)
}

// PrepareQueryParameters applies query-local overrides. An absent override
// preserves module-configured pre-query state unless a previous local override
// must be cleared. Explicit parameter updates use SetQueryParameters.
func (e *PropositionalEngine) PrepareQueryParameters(params *uparams.ParamsRef) error {
	if err := e.open(); err != nil {
		return err
	}
	if params == nil && !e.queryParamsReady {
		return nil
	}
	return e.SetQueryParameters(params)
}

// SetQueryParameters retains a query-local parameter snapshot. A nil value
// clears the previous snapshot so module parameters become the sole source.
func (e *PropositionalEngine) SetQueryParameters(params *uparams.ParamsRef) (result error) {
	if err := e.open(); err != nil {
		return err
	}
	// Source updt_params replaces backend contexts inside the existing
	// engine_base. Preserve that lifetime even when Go rebuilds lazily:
	// clearing a prequery PDR/Spacer owner must not make Pop legal again.
	e.engineInstantiated = e.engineInstantiated || e.pdr != nil || e.spacer != nil
	e.invalidateQueryForMutation()
	if err := e.clearQueryResources(); err != nil {
		return err
	}
	// PDR and Spacer updt_params rebuild their contexts so constructor
	// policy snapshots cannot survive a parameter update.
	if err := e.closePDRForReplacement(); err != nil {
		return err
	}
	if err := e.resetSpacerForParameterUpdate(); err != nil {
		return err
	}
	result = e.clearQueryParameters()
	if result == nil && params != nil {
		e.queryParams = e.params.EmptyValue()
		result = e.params.Assign(&e.queryParams, params)
		e.queryParamsReady = result == nil
		if result != nil {
			result = errors.Join(result, e.clearQueryParameters())
		}
	}
	if result == nil {
		e.revision++
	}
	return result
}

// SelectConfiguredEngine resolves query-local :engine before the fixedpoint
// module setting and source auto-config default, then keeps that source choice.
// Explicit SelectEngine is available when a Go caller deliberately switches.
func (e *PropositionalEngine) SelectConfiguredEngine() (result error) {
	if err := e.open(); err != nil {
		return err
	}
	// context::configure_engine keeps its first resolved engine type.
	// Explicit SelectEngine remains the Go API's deliberate override.
	if e.engineSelectionSet {
		return nil
	}

	view, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, view.Close()) }()
	selection, err := view.Symbol("engine", usymbol.Symbol{})
	if err != nil {
		return err
	}
	name := "auto-config"
	if selection.Value != 0 {
		name, _, err = e.params.SymbolsField.BareString(selection)
		if err != nil {
			return err
		}
	}
	// configure_engine recognizes explicit names and otherwise leaves its
	// engine type unset, entering automatic classification. This includes
	// unknown parameter symbols as well as the default "auto-config".
	if _, err := parseEngineKind(name); err != nil {
		name = "auto-config"
	}
	return e.SelectEngine(name)
}
