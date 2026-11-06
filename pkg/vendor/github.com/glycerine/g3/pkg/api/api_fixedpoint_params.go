package api

// Typed names for fixedpoint parameters consumed by retained Datalog, PDR,
// Spacer, and Horn preprocessing paths.

import (
	"errors"
	"fmt"
	"strings"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

type FixedpointBoolParameter string
type FixedpointUIntParameter string
type FixedpointDoubleParameter string
type FixedpointSymbolParameter string
type FixedpointStringParameter string

const (
	FixedpointPDRFarkas                  FixedpointBoolParameter = "fixedpoint.pdr.farkas"
	FixedpointPDRUseInductiveGeneralizer FixedpointBoolParameter = "fixedpoint.pdr.use_inductive_generalizer"
	FixedpointPDRUseMultiCoreGeneralizer FixedpointBoolParameter = "fixedpoint.pdr.use_multicore_generalizer"
	FixedpointPDRUseArithGeneralizer     FixedpointBoolParameter = "fixedpoint.pdr.use_arith_inductive_generalizer"
	FixedpointPDRInductiveReachability   FixedpointBoolParameter = "fixedpoint.pdr.inductive_reachability_check"
	FixedpointPDRUseConvexClosure        FixedpointBoolParameter = "fixedpoint.pdr.use_convex_closure_generalizer"
	FixedpointPDRUseConvexInterior       FixedpointBoolParameter = "fixedpoint.pdr.use_convex_interior_generalizer"
	FixedpointPDRUTVPI                   FixedpointBoolParameter = "fixedpoint.pdr.utvpi"
	FixedpointPDRUseModelGeneralizer     FixedpointBoolParameter = "fixedpoint.pdr.use_model_generalizer"
	FixedpointPDRBreadthFirst            FixedpointBoolParameter = "fixedpoint.pdr.bfs_model_search"
	FixedpointPDRFlexibleTrace           FixedpointBoolParameter = "fixedpoint.pdr.flexible_trace"
	FixedpointDatalogSubsumption         FixedpointBoolParameter = "fixedpoint.datalog.subsumption"
	FixedpointTransformSlice             FixedpointBoolParameter = "fixedpoint.xform.slice"
	FixedpointTransformInlineLinear      FixedpointBoolParameter = "fixedpoint.xform.inline_linear"
	FixedpointTransformInlineEager       FixedpointBoolParameter = "fixedpoint.xform.inline_eager"
	FixedpointTransformBitBlast          FixedpointBoolParameter = "fixedpoint.xform.bit_blast"
	FixedpointTransformMagic             FixedpointBoolParameter = "fixedpoint.xform.magic"
	FixedpointTransformScale             FixedpointBoolParameter = "fixedpoint.xform.scale"
	FixedpointTransformKarr              FixedpointBoolParameter = "fixedpoint.xform.karr"
	FixedpointSpacerSkipPropagate        FixedpointBoolParameter = "fixedpoint.spacer.skip_propagate"
	FixedpointSpacerResetObligations     FixedpointBoolParameter = "fixedpoint.spacer.reset_obligation_queue"
	FixedpointSpacerInitializeReach      FixedpointBoolParameter = "fixedpoint.spacer.init_reach_facts"
	FixedpointSpacerEliminateAuxiliary   FixedpointBoolParameter = "fixedpoint.spacer.elim_aux"
	FixedpointSpacerReachDNF             FixedpointBoolParameter = "fixedpoint.spacer.reach_dnf"
	FixedpointSpacerRestarts             FixedpointBoolParameter = "fixedpoint.spacer.restarts"
	FixedpointSpacerInstantiate          FixedpointBoolParameter = "fixedpoint.spacer.instantiate"
	FixedpointSpacerQuantifiedLemmas     FixedpointBoolParameter = "fixedpoint.spacer.qlemmas"
	FixedpointSpacerGroundCTI            FixedpointBoolParameter = "fixedpoint.spacer.ground_cti"
	FixedpointSpacerWeakAbstraction      FixedpointBoolParameter = "fixedpoint.spacer.weak_abs"
)

const (
	FixedpointTimeout               FixedpointUIntParameter = "fixedpoint.timeout"
	FixedpointPDRMaximumContexts    FixedpointUIntParameter = "fixedpoint.pdr.max_num_contexts"
	FixedpointPDRCacheMode          FixedpointUIntParameter = "fixedpoint.pdr.cache_mode"
	FixedpointPDRFlexibleTraceDepth FixedpointUIntParameter = "fixedpoint.pdr.flexible_trace_depth"
	FixedpointSpacerMaximumLevel    FixedpointUIntParameter = "fixedpoint.spacer.max_level"
	FixedpointSpacerOrderChildren   FixedpointUIntParameter = "fixedpoint.spacer.order_children"
	FixedpointSpacerRestartInitial  FixedpointUIntParameter = "fixedpoint.spacer.restart_initial_threshold"
	FixedpointSpacerRandomSeed      FixedpointUIntParameter = "fixedpoint.spacer.random_seed"
)

const FixedpointEngine FixedpointSymbolParameter = "fixedpoint.engine"

// FixedpointQueryOptions contains typed settings applied to one query. Names
// are validated against the live fixedpoint catalog, so explicitly converted
// custom names cannot silently acquire the wrong parameter kind.
type FixedpointQueryOptions struct {
	Bool   map[FixedpointBoolParameter]bool
	UInt   map[FixedpointUIntParameter]uint32
	Double map[FixedpointDoubleParameter]float64
	Symbol map[FixedpointSymbolParameter]string
	String map[FixedpointStringParameter]string
}

func fixedpointQueryParameterName(name string) string {
	name = strings.TrimPrefix(name, "fixedpoint.")
	return uparams.NormParamName(&name)
}

func (c *SMTContext) fixedpointQueryParameters(options *FixedpointQueryOptions) (result uparams.ParamsRef, context *uparams.ParamsContext, resultErr error) {
	if err := c.open(); err != nil {
		return uparams.ParamsRef{}, nil, err
	}
	service, err := c.command.GlobalParameters()
	if err != nil {
		return uparams.ParamsRef{}, nil, err
	}
	context = service.Context
	result, err = context.NewValue()
	if err != nil {
		return uparams.ParamsRef{}, nil, err
	}
	owner, allocated := context, result
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, owner.Del(&allocated))
			result = uparams.ParamsRef{}
			context = nil
		}
	}()
	descriptions, err := service.SourceModuleDescriptorsName("fixedpoint")
	if err != nil {
		return uparams.ParamsRef{}, nil, err
	}
	check := func(name string, expected uparams.ParamKind) (string, error) {
		name = fixedpointQueryParameterName(name)
		kind, x := descriptions.KindName(name)
		if x != nil {
			return "", x
		}
		if kind != expected {
			return "", fmt.Errorf("api: fixedpoint query parameter %s has kind %s, expected %s", name, kind.String(), expected.String())
		}
		return name, nil
	}
	for parameter, value := range options.Bool {
		name, x := check(string(parameter), uparams.ParamBool)
		if x != nil {
			return uparams.ParamsRef{}, nil, x
		}
		if x = context.SetBoolName(&result, &name, value); x != nil {
			return uparams.ParamsRef{}, nil, x
		}
	}
	for parameter, value := range options.UInt {
		name, x := check(string(parameter), uparams.ParamUInt)
		if x != nil {
			return uparams.ParamsRef{}, nil, x
		}
		if x = context.SetUIntName(&result, &name, value); x != nil {
			return uparams.ParamsRef{}, nil, x
		}
	}
	for parameter, value := range options.Double {
		name, x := check(string(parameter), uparams.ParamDouble)
		if x != nil {
			return uparams.ParamsRef{}, nil, x
		}
		if x = context.SetDoubleName(&result, &name, value); x != nil {
			return uparams.ParamsRef{}, nil, x
		}
	}
	for parameter, value := range options.Symbol {
		name, x := check(string(parameter), uparams.ParamSymbol)
		if x != nil {
			return uparams.ParamsRef{}, nil, x
		}
		symbol, x := context.SymbolsField.Intern(value)
		if x != nil {
			return uparams.ParamsRef{}, nil, x
		}
		if x = context.SetSymbolName(&result, &name, symbol); x != nil {
			return uparams.ParamsRef{}, nil, x
		}
	}
	for parameter, value := range options.String {
		name, x := check(string(parameter), uparams.ParamString)
		if x != nil {
			return uparams.ParamsRef{}, nil, x
		}
		if x = context.SetStringName(&result, &name, &value); x != nil {
			return uparams.ParamsRef{}, nil, x
		}
	}
	committed = true
	return result, context, nil
}

func (c *SMTContext) SetFixedpointBool(parameter FixedpointBoolParameter, value bool) error {
	return c.SetBoolOption(string(parameter), value)
}

func (c *SMTContext) SetFixedpointUInt(parameter FixedpointUIntParameter, value uint32) error {
	return c.SetUIntOption(string(parameter), value)
}

func (c *SMTContext) SetFixedpointSymbol(parameter FixedpointSymbolParameter, value string) error {
	return c.SetSymbolOption(string(parameter), value)
}
