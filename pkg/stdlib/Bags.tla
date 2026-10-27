--------------------------- MODULE Bags -----------------------------------
EXTENDS Integers, Apalache

\* @typeAlias: BAG = a -> Int;
LOCAL __ALIAS == TRUE

\* @type: (a -> Int) => Bool;
IsABag(B) == \A x \in DOMAIN B: B[x] > 0

\* @type: (a -> Int) => Set(a);
BagToSet(B) == DOMAIN B

\* @type: Set(a) => a -> Int;
SetToBag(S) == [x \in S |-> 1]

\* @type: (a, a -> Int) => Bool;
BagIn(x, B) == x \in DOMAIN B

\* @type: () => a -> Int;
EmptyBag == [x \in {} |-> 0]

\* @type: (a -> Int, a -> Int) => a -> Int;
B1 (+) B2 ==
  [x \in DOMAIN B1 \union DOMAIN B2 |->
    (IF x \in DOMAIN B1 THEN B1[x] ELSE 0) +
    (IF x \in DOMAIN B2 THEN B2[x] ELSE 0)]

\* @type: (a -> Int, a -> Int) => a -> Int;
B1 (-) B2 ==
  LET WithZeros == [x \in DOMAIN B1 |->
        B1[x] - (IF x \in DOMAIN B2 THEN B2[x] ELSE 0)]
  IN [x \in {x \in DOMAIN WithZeros: WithZeros[x] > 0} |-> WithZeros[x]]

\* @type: Set(a -> Int) => a -> Int;
BagUnion(S) ==
  LET Plus(B1, B2) == B1 (+) B2
  IN ApaFoldSet(Plus, EmptyBag, S)

\* @type: (a -> Int, a -> Int) => Bool;
B1 \sqsubseteq B2 ==
  /\ DOMAIN B1 \subseteq DOMAIN B2
  /\ \A x \in DOMAIN B1: B1[x] \leq B2[x]

\* @type: (a -> Int) => Set(a -> Int);
SubBag(B) ==
  LET All == UNION {[S -> {n \in Nat: n > 0}]: S \in SUBSET DOMAIN B}
  IN {SB \in All: \A x \in DOMAIN SB: SB[x] \leq B[x]}

\* @type: ((a) => b, a -> Int) => b -> Int;
BagOfAll(F(_), B) ==
  LET Extend(partial, x) == partial (+) [y \in {F(x)} |-> B[x]]
  IN ApaFoldSet(Extend, EmptyBag, DOMAIN B)

\* @type: (a -> Int) => Int;
BagCardinality(B) ==
  LET Count(n, x) == n + B[x]
  IN ApaFoldSet(Count, 0, DOMAIN B)

\* @type: (a, a -> Int) => Int;
CopiesIn(x, B) == IF x \in DOMAIN B THEN B[x] ELSE 0
=============================================================================
