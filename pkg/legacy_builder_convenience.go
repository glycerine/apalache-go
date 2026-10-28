package apalache

// The methods in this file preserve the original lir.Builder convenience
// surface. They intentionally construct only the legacy AST; arity and type
// checks happen in the selected predefs compiler, as in Scala.

func (builder *LegacyBuilder) op(name string, args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.Operator(Op(name), args...)
}
func prependLegacy(head []LegacyBuilderExpr, tail []LegacyBuilderExpr) []LegacyBuilderExpr {
	result := make([]LegacyBuilderExpr, 0, len(head)+len(tail))
	result = append(result, head...)
	return append(result, tail...)
}

func (builder *LegacyBuilder) Eql(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("EQ", a, b)
}
func (builder *LegacyBuilder) Neql(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("NE", a, b)
}
func (builder *LegacyBuilder) AppOp(fn LegacyBuilderExpr, args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("OPER_APP", prependLegacy([]LegacyBuilderExpr{fn}, args)...)
}
func (builder *LegacyBuilder) Choose(variable, predicate LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("CHOOSE2", variable, predicate)
}
func (builder *LegacyBuilder) ChooseIn(variable, set, predicate LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("CHOOSE3", variable, set, predicate)
}
func (builder *LegacyBuilder) Guess(set LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!Guess", set)
}
func (builder *LegacyBuilder) Label(expr LegacyBuilderExpr, name string, args ...string) LegacyBuilderExpr {
	all := []LegacyBuilderExpr{expr, builder.String(name)}
	for _, arg := range args {
		all = append(all, builder.String(arg))
	}
	return builder.op("LABEL", all...)
}

func (builder *LegacyBuilder) And(args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("AND", args...)
}
func (builder *LegacyBuilder) Or(args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("OR", args...)
}
func (builder *LegacyBuilder) Not(a LegacyBuilderExpr) LegacyBuilderExpr { return builder.op("NOT", a) }
func (builder *LegacyBuilder) Impl(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("IMPLIES", a, b)
}
func (builder *LegacyBuilder) Equiv(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("EQUIV", a, b)
}
func (builder *LegacyBuilder) Forall(variable, predicate LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FORALL2", variable, predicate)
}
func (builder *LegacyBuilder) ForallIn(variable, set, predicate LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FORALL3", variable, set, predicate)
}
func (builder *LegacyBuilder) Exists(variable, predicate LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("EXISTS2", variable, predicate)
}
func (builder *LegacyBuilder) ExistsIn(variable, set, predicate LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("EXISTS3", variable, set, predicate)
}

func (builder *LegacyBuilder) Prime(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("PRIME", a)
}
func (builder *LegacyBuilder) PrimeEq(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.Eql(builder.Prime(a), b)
}
func (builder *LegacyBuilder) Stutt(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("STUTTER", a, b)
}
func (builder *LegacyBuilder) NoStutt(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("NO_STUTTER", a, b)
}
func (builder *LegacyBuilder) Enabled(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("ENABLED", a)
}
func (builder *LegacyBuilder) Unchanged(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("UNCHANGED", a)
}
func (builder *LegacyBuilder) UnchangedTuple(args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.Unchanged(builder.Tuple(args...))
}
func (builder *LegacyBuilder) Comp(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("COMPOSE", a, b)
}

func (builder *LegacyBuilder) CaseAny(firstGuard, firstEffect LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	name := "CASE"
	if len(rest)%2 == 1 {
		name = "CASE_OTHER"
	}
	return builder.op(name, prependLegacy([]LegacyBuilderExpr{firstGuard, firstEffect}, rest)...)
}
func (builder *LegacyBuilder) CaseSplit(firstGuard, firstEffect LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("CASE", prependLegacy([]LegacyBuilderExpr{firstGuard, firstEffect}, rest)...)
}
func (builder *LegacyBuilder) CaseOther(other, firstGuard, firstEffect LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("CASE_OTHER", prependLegacy([]LegacyBuilderExpr{other, firstGuard, firstEffect}, rest)...)
}
func (builder *LegacyBuilder) Ite(c, t, e LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("IF_THEN_ELSE", c, t, e)
}

func (builder *LegacyBuilder) AA(v, f LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("TEMPORAL_FORALL", v, f)
}
func (builder *LegacyBuilder) EE(v, f LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("TEMPORAL_EXISTS", v, f)
}
func (builder *LegacyBuilder) Box(f LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("GLOBALLY", f)
}
func (builder *LegacyBuilder) Diamond(f LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("EVENTUALLY", f)
}
func (builder *LegacyBuilder) Guarantees(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("GUARANTEES", a, b)
}
func (builder *LegacyBuilder) LeadsTo(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("LEADS_TO", a, b)
}
func (builder *LegacyBuilder) SF(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("STRONG_FAIRNESS", a, b)
}
func (builder *LegacyBuilder) WF(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("WEAK_FAIRNESS", a, b)
}

func (builder *LegacyBuilder) Plus(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("PLUS", a, b)
}
func (builder *LegacyBuilder) Minus(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("MINUS", a, b)
}
func (builder *LegacyBuilder) UMinus(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("UNARY_MINUS", a)
}
func (builder *LegacyBuilder) Mult(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("MULT", a, b)
}
func (builder *LegacyBuilder) Div(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("DIV", a, b)
}
func (builder *LegacyBuilder) Mod(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("MOD", a, b)
}
func (builder *LegacyBuilder) RealDiv(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("REAL_DIV", a, b)
}
func (builder *LegacyBuilder) Exp(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("POW", a, b)
}
func (builder *LegacyBuilder) DotDot(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("INT_RANGE", a, b)
}
func (builder *LegacyBuilder) Lt(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("LT", a, b)
}
func (builder *LegacyBuilder) Gt(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("GT", a, b)
}
func (builder *LegacyBuilder) Le(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("LE", a, b)
}
func (builder *LegacyBuilder) Ge(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("GE", a, b)
}

func (builder *LegacyBuilder) Card(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FiniteSets!Cardinality", a)
}
func (builder *LegacyBuilder) IsFinite(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FiniteSets!IsFiniteSet", a)
}
func (builder *LegacyBuilder) AppFun(fn, arg LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FUN_APP", fn, arg)
}
func (builder *LegacyBuilder) Domain(fn LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("DOMAIN", fn)
}
func (builder *LegacyBuilder) Record(firstKey, firstValue LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("RECORD", prependLegacy([]LegacyBuilderExpr{firstKey, firstValue}, rest)...)
}
func (builder *LegacyBuilder) Except(fn, firstKey, firstValue LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("EXCEPT", prependLegacy([]LegacyBuilderExpr{fn, firstKey, firstValue}, rest)...)
}
func (builder *LegacyBuilder) FunDef(body, firstVar, firstSet LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FUN_CTOR", prependLegacy([]LegacyBuilderExpr{body, firstVar, firstSet}, rest)...)
}
func (builder *LegacyBuilder) RecursiveFunDef(body, firstVar, firstSet LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FUN_REC_CTOR", prependLegacy([]LegacyBuilderExpr{body, firstVar, firstSet}, rest)...)
}
func (builder *LegacyBuilder) RecursiveFunRef() LegacyBuilderExpr { return builder.op("FUN_REC_REF") }
func (builder *LegacyBuilder) Tuple(args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("TUPLE", args...)
}

func (builder *LegacyBuilder) Append(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Sequences!Append", a, b)
}
func (builder *LegacyBuilder) Concat(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Sequences!Concat", a, b)
}
func (builder *LegacyBuilder) Head(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Sequences!Head", a)
}
func (builder *LegacyBuilder) Tail(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Sequences!Tail", a)
}
func (builder *LegacyBuilder) Len(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Sequences!Len", a)
}
func (builder *LegacyBuilder) SubSeq(a, from, to LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Sequences!SubSeq", a, from, to)
}

func (builder *LegacyBuilder) EnumSet(args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_ENUM", args...)
}
func (builder *LegacyBuilder) EmptySet() LegacyBuilderExpr { return builder.EnumSet() }
func (builder *LegacyBuilder) In(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_IN", a, b)
}
func (builder *LegacyBuilder) NotIn(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_NOT_IN", a, b)
}
func (builder *LegacyBuilder) Cap(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_INTERSECT", a, b)
}
func (builder *LegacyBuilder) Cup(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_UNION2", a, b)
}
func (builder *LegacyBuilder) Union(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_UNARY_UNION", a)
}
func (builder *LegacyBuilder) Filter(v, set, p LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_FILTER", v, set, p)
}
func (builder *LegacyBuilder) Map(body, firstVar, firstSet LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_MAP", prependLegacy([]LegacyBuilderExpr{body, firstVar, firstSet}, rest)...)
}
func (builder *LegacyBuilder) FunSet(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("FUN_SET", a, b)
}
func (builder *LegacyBuilder) RecordSet(firstName, firstSet LegacyBuilderExpr, rest ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("RECORD_SET", prependLegacy([]LegacyBuilderExpr{firstName, firstSet}, rest)...)
}
func (builder *LegacyBuilder) SeqSet(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Sequences!Seq", a)
}
func (builder *LegacyBuilder) SubsetEq(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_SUBSET_EQ", a, b)
}
func (builder *LegacyBuilder) SetMinus(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_MINUS", a, b)
}
func (builder *LegacyBuilder) Times(args ...LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_TIMES", args...)
}
func (builder *LegacyBuilder) PowerSet(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("SET_POWERSET", a)
}
func (builder *LegacyBuilder) PrimeInSingleton(name, element LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.In(builder.Prime(name), builder.EnumSet(element))
}

func (builder *LegacyBuilder) Assign(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!:=", a, b)
}
func (builder *LegacyBuilder) AssignPrime(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.Assign(builder.Prime(a), b)
}
func (builder *LegacyBuilder) Expand(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!Expand", a)
}
func (builder *LegacyBuilder) SetAsFun(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!SetAsFun", a)
}
func (builder *LegacyBuilder) MkSeq(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!MkSeq", a, b)
}
func (builder *LegacyBuilder) Skolem(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!Skolem", a)
}
func (builder *LegacyBuilder) ConstCardinality(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!ConstCardinality", a)
}
func (builder *LegacyBuilder) SelectInSet(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!SelectInSet", a, b)
}
func (builder *LegacyBuilder) SelectInFun(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!SelectInFun", a, b)
}
func (builder *LegacyBuilder) StoreInSet(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!StoreInSet", a, b)
}
func (builder *LegacyBuilder) StoreInFun(a, b, c LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!StoreInSet", a, b, c)
}
func (builder *LegacyBuilder) StoreNotInSet(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!storeNotInSet", a, b)
}
func (builder *LegacyBuilder) StoreNotInFun(a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!storeNotInFun", a, b)
}
func (builder *LegacyBuilder) SMTMap(operator Operator, a, b LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.Operator(NewSMTMapOperator(operator), a, b)
}
func (builder *LegacyBuilder) UnconstrainArray(a LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Apalache!UnconstrainArray", a)
}

func (builder *LegacyBuilder) Variant(tag string, value LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Variants!Variant", builder.String(tag), value)
}
func (builder *LegacyBuilder) VariantFilter(tag string, set LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Variants!VariantFilter", builder.String(tag), set)
}
func (builder *LegacyBuilder) VariantTag(value LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Variants!VariantTag", value)
}
func (builder *LegacyBuilder) VariantGetUnsafe(tag string, value LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Variants!VariantGetUnsafe", builder.String(tag), value)
}
func (builder *LegacyBuilder) VariantGetOrElse(tag string, value, fallback LegacyBuilderExpr) LegacyBuilderExpr {
	return builder.op("Variants!VariantGetOrElse", builder.String(tag), value, fallback)
}
