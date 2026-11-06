package api

// Source: src/api/api_tactic.cpp:48-67,286-322;
// src/tactic/probe.h:64-126. Only probes backed by retained concrete Go
// evaluators are registered here.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// APIProbeDescriptor is the stable public metadata associated with a named
// probe. It is a value so callers never borrow registry storage.
type APIProbeDescriptor = tactic.ProbeDescriptor

type apiProbeRegistration struct {
	descriptor APIProbeDescriptor
	kind       tactic.ProbeKind
}

func apiProbeRegistrationByName(name string) (apiProbeRegistration, bool) {
	name = apiTacticName(name)
	for _, descriptor := range tactic.BuiltinProbeDescriptors() {
		if descriptor.Name == name {
			return apiProbeRegistration{descriptor: descriptor, kind: apiProbeKind(name)}, true
		}
	}
	return apiProbeRegistration{}, false
}

func apiProbeKind(name string) tactic.ProbeKind {
	switch name {
	case "memory":
		return tactic.ProbeMemory
	case "depth":
		return tactic.ProbeDepth
	case "size":
		return tactic.ProbeSize
	case "num-exprs":
		return tactic.ProbeNumExprs
	case "num-consts":
		return tactic.ProbeNumConsts
	case "num-bool-consts":
		return tactic.ProbeNumBoolConsts
	case "num-arith-consts":
		return tactic.ProbeNumArithmeticConsts
	case "num-bv-consts":
		return tactic.ProbeNumBitVectorConsts
	case "produce-proofs":
		return tactic.ProbeProduceProofs
	case "produce-model":
		return tactic.ProbeProduceModels
	case "produce-unsat-cores":
		return tactic.ProbeProduceUnsatCores
	case "has-quantifiers":
		return tactic.ProbeHasQuantifier
	case "has-patterns":
		return tactic.ProbeHasPattern
	case "is-propositional":
		return tactic.ProbeIsPropositional
	case "is-qfbv":
		return tactic.ProbeIsQFBitVector
	case "is-qfaufbv":
		return tactic.ProbeIsQFArrayUFBitVector
	default:
		return tactic.ProbeConstant
	}
}

// ProbeCount returns the number of retained named probes in native
// registration order.
func (c *SMTContext) ProbeCount() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return uint32(len(tactic.BuiltinProbeDescriptors())), nil
}

// ProbeDescriptor returns copied metadata at a native registration index.
func (c *SMTContext) ProbeDescriptor(index uint32) (APIProbeDescriptor, error) {
	if err := c.open(); err != nil {
		return APIProbeDescriptor{}, err
	}
	descriptors := tactic.BuiltinProbeDescriptors()
	if uint64(index) >= uint64(len(descriptors)) {
		return APIProbeDescriptor{}, arena.ErrBounds
	}
	return descriptors[index], nil
}

// ProbeDescription returns the retained source description for a name.
func (c *SMTContext) ProbeDescription(name string) (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	registration, found := apiProbeRegistrationByName(name)
	if !found {
		return "", ast.ErrASTDomain
	}
	return registration.descriptor.Description, nil
}
