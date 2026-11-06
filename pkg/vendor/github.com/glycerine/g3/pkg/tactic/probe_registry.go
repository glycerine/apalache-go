package tactic

// ProbeDescriptor is the source name and description shared by tactic help
// output and the public probe registry.
type ProbeDescriptor struct {
	Name        string
	Description string
}

// builtinProbeDescriptors follows generated ADD_PROBE registration order.
var builtinProbeDescriptors = [...]ProbeDescriptor{
	{"ackr-bound-probe", "A probe to give an upper bound of Ackermann congruence lemmas that a formula might generate."},
	{"is-unbounded", "true if the goal contains integer/real constants that do not have lower/upper bounds."},
	{"is-pb", "true if the goal is a pseudo-boolean problem."},
	{"arith-max-deg", "max polynomial total degree of an arithmetic atom."},
	{"arith-avg-deg", "avg polynomial total degree of an arithmetic atom."},
	{"arith-max-bw", "max coefficient bit width."},
	{"arith-avg-bw", "avg coefficient bit width."},
	{"is-qflia", "true if the goal is in QF_LIA."},
	{"is-qfauflia", "true if the goal is in QF_AUFLIA."},
	{"is-qflra", "true if the goal is in QF_LRA."},
	{"is-qflira", "true if the goal is in QF_LIRA."},
	{"is-ilp", "true if the goal is ILP."},
	{"is-qfnia", "true if the goal is in QF_NIA (quantifier-free nonlinear integer arithmetic)."},
	{"is-qfnra", "true if the goal is in QF_NRA (quantifier-free nonlinear real arithmetic)."},
	{"is-nia", "true if the goal is in NIA (nonlinear integer arithmetic, formula may have quantifiers)."},
	{"is-nra", "true if the goal is in NRA (nonlinear real arithmetic, formula may have quantifiers)."},
	{"is-nira", "true if the goal is in NIRA (nonlinear integer and real arithmetic, formula may have quantifiers)."},
	{"is-lia", "true if the goal is in LIA (linear integer arithmetic, formula may have quantifiers)."},
	{"is-lra", "true if the goal is in LRA (linear real arithmetic, formula may have quantifiers)."},
	{"is-lira", "true if the goal is in LIRA (linear integer and real arithmetic, formula may have quantifiers)."},
	{"is-qfufnra", "true if the goal is QF_UFNRA (quantifier-free nonlinear real arithmetic with other theories)."},
	{"is-qfbv-eq", "true if the goal is in a fragment of QF_BV which uses only =, extract, concat."},
	{"is-qffp", "true if the goal is in QF_FP (floats)."},
	{"is-qffpbv", "true if the goal is in QF_FPBV (floats+bit-vectors)."},
	{"is-qffplra", "true if the goal is in QF_FPLRA."},
	{"memory", "amount of used memory in megabytes."},
	{"depth", "depth of the input goal."},
	{"size", "number of assertions in the given goal."},
	{"num-exprs", "number of expressions/terms in the given goal."},
	{"num-consts", "number of non Boolean constants in the given goal."},
	{"num-bool-consts", "number of Boolean constants in the given goal."},
	{"num-arith-consts", "number of arithmetic constants in the given goal."},
	{"num-bv-consts", "number of bit-vector constants in the given goal."},
	{"produce-proofs", "true if proof generation is enabled for the given goal."},
	{"produce-model", "true if model generation is enabled for the given goal."},
	{"produce-unsat-cores", "true if unsat-core generation is enabled for the given goal."},
	{"has-quantifiers", "true if the goal contains quantifiers."},
	{"has-patterns", "true if the goal contains quantifiers with patterns."},
	{"is-propositional", "true if the goal is in propositional logic."},
	{"is-qfbv", "true if the goal is in QF_BV."},
	{"is-qfaufbv", "true if the goal is in QF_AUFBV."},
	{"is-quasi-pb", "true if the goal is quasi-pb."},
}

// BuiltinProbeDescriptors returns the source-ordered built-in probe metadata.
func BuiltinProbeDescriptors() []ProbeDescriptor {
	result := make([]ProbeDescriptor, len(builtinProbeDescriptors))
	copy(result, builtinProbeDescriptors[:])
	return result
}
