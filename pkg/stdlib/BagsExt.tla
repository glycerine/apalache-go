--------------------------- MODULE BagsExt -----------------------------------
EXTENDS Integers, Bags, Apalache

\* @type: (a -> Int, a) => a -> Int;
BagAdd(B, x) ==
  IF x \in DOMAIN B
  THEN [e \in DOMAIN B |-> IF e = x THEN B[e] + 1 ELSE B[e]]
  ELSE [e \in DOMAIN B \union {x} |-> IF e = x THEN 1 ELSE B[e]]

\* @type: (a -> Int, a) => a -> Int;
BagRemove(B, x) ==
  IF x \in DOMAIN B
  THEN IF B[x] = 1
       THEN [e \in DOMAIN B \ {x} |-> B[e]]
       ELSE [e \in DOMAIN B |-> IF e = x THEN B[e] - 1 ELSE B[e]]
  ELSE B

\* @type: (a -> Int, a) => a -> Int;
BagRemoveAll(B, x) == [e \in DOMAIN B \ {x} |-> B[e]]

\* @type: (Int -> Int) => Int;
SumBag(B) ==
  LET Add(acc, x) == acc + x * B[x]
  IN ApaFoldSet(Add, 0, DOMAIN B)

\* @type: (Int -> Int) => Int;
ProductBag(B) ==
  LET Multiply(acc, x) == acc * (x ^ B[x])
  IN ApaFoldSet(Multiply, 1, DOMAIN B)
=============================================================================
