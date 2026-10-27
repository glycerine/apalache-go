--------------------------- MODULE Functions ---------------------------------
EXTENDS Integers, FiniteSets, Apalache

\* @type: (a -> b, Set(a)) => a -> b;
Restrict(f, S) == [x \in S |-> f[x]]

\* @type: (a -> b) => Set(b);
Range(f) == {f[x] : x \in DOMAIN f}

\* @type: (a -> b, Set(a), Set(b)) => b -> a;
Inverse(f, S, T) ==
  [t \in T |-> CHOOSE s \in S: t \in Range(f) => f[s] = t]

\* @type: (a -> b) => b -> a;
AntiFunction(f) == Inverse(f, DOMAIN f, Range(f))

\* @type: (a -> b) => Bool;
IsInjective(f) ==
  Cardinality(Range(f)) = Cardinality(DOMAIN f)

\* @type: (Set(a), Set(b)) => Set(a -> b);
Injection(S, T) == {f \in [S -> T]: IsInjective(f)}

\* @type: (Set(a), Set(b), a -> b) => Bool;
IsSurjective(S, T, f) ==
  \A t \in T: \E s \in S: f[s] = t

\* @type: (Set(a), Set(b)) => Set(a -> b);
Surjection(S, T) ==
  {f \in [S -> T]: IsSurjective(S, T, f)}

\* @type: (Set(a), Set(b)) => Set(a -> b);
Bijection(S, T) ==
  {f \in [S -> T]: IsSurjective(S, T, f) /\ IsInjective(f)}

\* @type: (Set(a), Set(b)) => Bool;
ExistsInjection(S, T) == Cardinality(S) <= Cardinality(T)

\* @type: (Set(a), Set(b)) => Bool;
ExistsSurjection(S, T) == Cardinality(S) >= Cardinality(T)

\* @type: (Set(a), Set(b)) => Bool;
ExistsBijection(S, T) == Cardinality(S) = Cardinality(T)

\* @type: ((b, c) => c, a -> b, c, a) => c;
__FoldFunctionStep(op(_, _), fun, acc, x) == op(fun[x], acc)

\* @type: ((b, c) => c, c, a -> b) => c;
FoldFunction(op(_, _), base, fun) ==
  LET Map(acc, x) == __FoldFunctionStep(op, fun, acc, x)
  IN ApaFoldSet(Map, base, DOMAIN fun)

\* @type: ((b, c) => c, c, a -> b, Set(a)) => c;
FoldFunctionOnSet(op(_, _), base, fun, indices) ==
  LET Map(acc, x) == __FoldFunctionStep(op, fun, acc, x)
  IN ApaFoldSet(Map, base, indices)
===============================================================================
