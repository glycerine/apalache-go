package rewriter

// Parameter collectors remain with the C++ definitions they translate.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func catalogInsert(d *uparams.ParamDescrs, key string, kind uparams.ParamKind, descr, def, module string) error {
	if module == "" {
		return d.InsertName(key, kind, &descr, &def, nil)
	}
	return d.InsertName(key, kind, &descr, &def, &module)
}

// Source: src/ast/rewriter/arith_rewriter_params.pyg
func catalogCollectArithRewriterParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "algebraic_number_evaluator", uparams.ParamBool, "simplify/evaluate expressions containing (algebraic) irrational numbers.", "true", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mul_to_power", uparams.ParamBool, "collpase (* t ... t) into (^ t k), it is ignored if expand_power is true.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "expand_power", uparams.ParamBool, "expand (^ t k) into (* t ... t) if  1 < k <= max_degree.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "expand_tan", uparams.ParamBool, "replace (tan x) with (/ (sin x) (cos x)).", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_degree", uparams.ParamUInt, "max degree of algebraic numbers (and power operators) processed by simplifier.", "64", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "sort_sums", uparams.ParamBool, "sort the arguments of + application.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "gcd_rounding", uparams.ParamBool, "use gcd rounding on integer arithmetic atoms.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith_lhs", uparams.ParamBool, "all monomials are moved to the left-hand-side, and the right-hand-side is just a constant.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "arith_ineq_lhs", uparams.ParamBool, "rewrite inequalities so that right-hand-side is a constant.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "elim_to_real", uparams.ParamBool, "eliminate to_real from arithmetic predicates that contain only integers.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "push_to_real", uparams.ParamBool, "distribute to_real over * and +.", "true", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "eq2ineq", uparams.ParamBool, "expand equalities into two inequalities", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "elim_rem", uparams.ParamBool, "replace (rem x y) with (ite (>= y 0) (mod x y) (- (mod x y))).", "false", "rewriter"); e != nil {
		return e
	}
	return nil
}

// Source: src/ast/rewriter/array_rewriter_params.pyg
func catalogCollectArrayRewriterParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "expand_select_store", uparams.ParamBool, "replace a (select (store ...) ...) term by an if-then-else term", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "expand_store_eq", uparams.ParamBool, "reduce (store ...) = (store ...) with a common base into selects", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "sort_store", uparams.ParamBool, "sort nested stores when the indices are known to be different", "false", "rewriter"); e != nil {
		return e
	}
	return nil
}

// Source: src/ast/rewriter/bool_rewriter_params.pyg
func catalogCollectBoolRewriterParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "ite_extra_rules", uparams.ParamBool, "extra ite simplifications, these additional simplifications may reduce size locally but increase globally", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "flat", uparams.ParamBool, "create nary applications for and,or,+,*,bvadd,bvmul,bvand,bvor,bvxor", "true", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "elim_and", uparams.ParamBool, "conjunctions are rewritten using negation and disjunctions", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "local_ctx", uparams.ParamBool, "perform local (i.e., cheap) context simplifications", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "local_ctx_limit", uparams.ParamUInt, "limit for applying local context simplifier", "4294967295", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "blast_distinct", uparams.ParamBool, "expand a distinct predicate into a quadratic number of disequalities", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "blast_distinct_threshold", uparams.ParamUInt, "when blast_distinct is true, only distinct expressions with less than this number of arguments are blasted", "4294967295", "rewriter"); e != nil {
		return e
	}
	return nil
}

// Source: src/ast/rewriter/bv_rewriter_params.pyg
func catalogCollectBvRewriterParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "udiv2mul", uparams.ParamBool, "convert constant udiv to mul", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "split_concat_eq", uparams.ParamBool, "split equalities of the form (= (concat t1 t2) t3)", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bit2bool", uparams.ParamBool, "try to convert bit-vector terms of size 1 into Boolean terms", "true", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "blast_eq_value", uparams.ParamBool, "blast (some) Bit-vector equalities into bits", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "elim_sign_ext", uparams.ParamBool, "expand sign-ext operator using concat and extract", "true", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "hi_div0", uparams.ParamBool, "use the 'hardware interpretation' for division by zero (for bit-vector terms)", "true", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "mul2concat", uparams.ParamBool, "replace multiplication by a power of two into a concatenation", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bvnot2arith", uparams.ParamBool, "replace (bvnot x) with (bvsub -1 x)", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_sort_ac", uparams.ParamBool, "sort the arguments of all AC operators", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_trailing", uparams.ParamBool, "lean removal of trailing zeros", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_extract_prop", uparams.ParamBool, "attempt to partially propagate extraction inwards", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_not_simpl", uparams.ParamBool, "apply simplifications for bvnot", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_ite2id", uparams.ParamBool, "rewrite ite that can be simplified to identity", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_le_extra", uparams.ParamBool, "additional bu_(u/s)le simplifications", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_urem_simpl", uparams.ParamBool, "additional simplification for bvurem", "false", "rewriter"); e != nil {
		return e
	}
	return nil
}

// Source: src/ast/rewriter/fpa_rewriter_params.pyg
func catalogCollectFpaRewriterParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "hi_fp_unspecified", uparams.ParamBool, "use the 'hardware interpretation' for unspecified values in fp.to_ubv, fp.to_sbv, fp.to_real, and fp.to_ieee_bv", "false", "rewriter"); e != nil {
		return e
	}
	return nil
}

// Source: src/ast/rewriter/poly_rewriter_params.pyg
func catalogCollectPolyRewriterParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "som", uparams.ParamBool, "put polynomials in som-of-monomials form", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "som_blowup", uparams.ParamUInt, "maximum number of monomials generated when putting a polynomial in sum-of-monomials normal form", "4294967295", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "hoist_mul", uparams.ParamBool, "hoist multiplication over summation to minimize number of multiplications", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "hoist_cmul", uparams.ParamBool, "hoist constant multiplication over summation to minimize number of multiplications", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "flat", uparams.ParamBool, "create nary applications for and,or,+,*,bvadd,bvmul,bvand,bvor,bvxor", "true", "rewriter"); e != nil {
		return e
	}
	return nil
}

// Source: src/ast/rewriter/rewriter_params.pyg
func catalogCollectRewriterParams(d *uparams.ParamDescrs) error {
	if e := catalogInsert(d, "max_memory", uparams.ParamUInt, "maximum amount of memory in megabytes", "4294967295", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "max_steps", uparams.ParamUInt, "maximum number of steps", "4294967295", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "flat", uparams.ParamBool, "create nary applications for and,or,+,*,bvadd,bvmul,bvand,bvor,bvxor", "true", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "push_ite_arith", uparams.ParamBool, "push if-then-else over arithmetic terms.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "push_ite_bv", uparams.ParamBool, "push if-then-else over bit-vector terms.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "pull_cheap_ite", uparams.ParamBool, "pull if-then-else terms when cheap.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "bv_ineq_consistency_test_max", uparams.ParamUInt, "max size of conjunctions on which to perform consistency test based on inequalities on bitvectors.", "0", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "cache_all", uparams.ParamBool, "cache all intermediate results.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "rewrite_patterns", uparams.ParamBool, "rewrite patterns.", "false", "rewriter"); e != nil {
		return e
	}
	if e := catalogInsert(d, "ignore_patterns_on_ground_qbody", uparams.ParamBool, "ignores patterns on quantifiers that don't mention their bound variables.", "true", "rewriter"); e != nil {
		return e
	}
	return nil
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectArithRewriterParams", catalogCollectArithRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectArrayRewriterParams", catalogCollectArrayRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectBoolRewriterParams", catalogCollectBoolRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectBvRewriterParams", catalogCollectBvRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectFpaRewriterParams", catalogCollectFpaRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectPolyRewriterParams", catalogCollectPolyRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectRewriterParams", catalogCollectRewriterParams)
}
