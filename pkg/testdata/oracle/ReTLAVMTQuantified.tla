-------------------------- MODULE ReTLAVMTQuantified --------------------------
EXTENDS Integers

CONSTANT
    \* @type: Set(A);
    S

VARIABLES
    \* @type: A;
    x,
    \* @type: Int;
    n,
    \* @type: Bool;
    flag,
    \* @type: <<A, Int>> -> A;
    f

Init ==
    /\ x = "a_OF_A"
    /\ n = 0
    /\ flag = TRUE
    /\ f = [a \in S, i \in Int |-> IF i = 0 THEN a ELSE x]

Next ==
    /\ x' = IF flag THEN "b_OF_A" ELSE x
    /\ n' = IF n = 0 THEN 1 ELSE 0
    /\ flag' = ~flag
    /\ f' = [f EXCEPT !["a_OF_A", 0] = "b_OF_A"]

Inv ==
    /\ (n = 0 \/ n = 1)
    /\ \E a \in S: a = x
    /\ \A i \in Int: i = i
    /\ (flag => f["a_OF_A", 0] = x)

===============================================================================
