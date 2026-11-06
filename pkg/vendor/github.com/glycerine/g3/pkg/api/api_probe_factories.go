package api

// Source: ADD_PROBE registrations consumed by install_tactics in the API
// context. Each branch constructs the already ported concrete source probe.

import (
	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
)

func (c *SMTContext) newRegisteredProbe(m *ast.ASTManager, registration apiProbeRegistration) (tactic.ProbeEvaluator, error) {
	globals, err := c.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	b, params := m.Budget(), globals.Context
	switch registration.descriptor.Name {
	case "ackr-bound-probe":
		return ackermannization.NewBoundProbe(b)
	case "is-unbounded":
		return arith.NewIsUnboundedProbe(b, params, globals)
	case "is-pb":
		return arith.NewIsPBProbe(b, params, globals)
	case "is-quasi-pb":
		return arith.NewQuasiPBProbe(b, params, globals)
	case "arith-max-deg":
		return arith.NewArithmeticMaximumDegreeProbe(b)
	case "arith-avg-deg":
		return arith.NewArithmeticAverageDegreeProbe(b)
	case "arith-max-bw":
		return arith.NewArithmeticMaximumBitWidthProbe(b)
	case "arith-avg-bw":
		return arith.NewArithmeticAverageBitWidthProbe(b)
	case "is-qfbv-eq":
		return bv.NewIsQFBVEqualityProbe(b, params, globals)
	case "is-qffp", "is-qffpbv":
		return smtlogics.NewIsQFFPProbe(b)
	case "is-qffplra":
		return smtlogics.NewIsQFFPLRAProbe(b)
	}
	var fragment arith.ArithmeticFragment
	switch registration.descriptor.Name {
	case "is-qflia":
		fragment = arith.FragmentQFLIA
	case "is-qfauflia":
		fragment = arith.FragmentQFAUFLIA
	case "is-qflra":
		fragment = arith.FragmentQFLRA
	case "is-qflira":
		fragment = arith.FragmentQFLIRA
	case "is-ilp":
		fragment = arith.FragmentILP
	case "is-qfnia":
		fragment = arith.FragmentQFNIA
	case "is-qfnra":
		fragment = arith.FragmentQFNRA
	case "is-nia":
		fragment = arith.FragmentNIA
	case "is-nra":
		fragment = arith.FragmentNRA
	case "is-nira":
		fragment = arith.FragmentNIRA
	case "is-lia":
		fragment = arith.FragmentLIA
	case "is-lra":
		fragment = arith.FragmentLRA
	case "is-lira":
		fragment = arith.FragmentLIRA
	case "is-qfufnra":
		fragment = arith.FragmentQFUFNRA
	default:
		return tactic.NewProbe(b, registration.kind, 0)
	}
	return arith.NewArithmeticFragmentProbe(b, params, globals, fragment)
}
