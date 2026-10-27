------------------------ MODULE FiniteSetsExt ---------------------------------
EXTENDS Integers, FiniteSets, Functions, Apalache

\* @type: ((a, b) => b, b, a) => b;
__FoldSetStep(op(_, _), acc, x) == op(x, acc)

\* @type: ((a, b) => b, b, Set(a)) => b;
FoldSet(op(_, _), base, set) ==
  LET Step(acc, x) == __FoldSetStep(op, acc, x)
  IN ApaFoldSet(Step, base, set)

\* @type: Set(Int) => Int;
SumSet(set) ==
  LET Plus(x, y) == x + y
  IN ApaFoldSet(Plus, 0, set)

\* @type: Set(Int) => Int;
ProductSet(set) ==
  LET Multiply(x, y) == x * y
  IN ApaFoldSet(Multiply, 1, set)

\* @type: ((a, b) => b, Set(a), b) => b;
ReduceSet(op(_, _), set, acc) == FoldSet(op, acc, set)

\* @type: Set(Set(a)) => Set(a);
FlattenSet(S) == UNION S

\* @type: (Set(a), Set(a)) => Set(a);
SymDiff(A, B) == {x \in A \union B: x \notin A \/ x \notin B}

\* @type: ((a) => Bool, Int, a) => Int;
__QuantifyStep(P(_), count, element) ==
  count + IF P(element) THEN 1 ELSE 0

\* @type: (Set(a), (a) => Bool) => Int;
Quantify(S, P(_)) ==
  LET Count(count, element) == __QuantifyStep(P, count, element)
  IN ApaFoldSet(Count, 0, S)

\* @type: (Int, Set(a)) => Set(Set(a));
kSubset(k, S) == {s \in SUBSET S: Cardinality(s) = k}

\* @type: Set(Int) => Int;
Max(S) == CHOOSE x \in S: \A y \in S: x >= y

\* @type: Set(Int) => Int;
Min(S) == CHOOSE x \in S: \A y \in S: x <= y

\* @type: Set(Set(a)) => Set(Set(a));
Choices(Sets) ==
  LET ChoiceFunction(Ts) ==
        {f \in [Ts -> UNION Ts]: \A T \in Ts: f[T] \in T}
  IN {Range(f): f \in ChoiceFunction(Sets)}

\* @type: (Set(a), (a) => Bool) => a;
ChooseUnique(S, P(_)) ==
  CHOOSE x \in S: P(x) /\ \A y \in S: P(y) => y = x
===============================================================================
