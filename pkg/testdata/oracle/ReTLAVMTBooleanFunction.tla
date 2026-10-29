----------------------- MODULE ReTLAVMTBooleanFunction -----------------------
EXTENDS Integers

VARIABLES
    \* @type: Bool;
    flag,
    \* @type: Int;
    n,
    \* @type: Int -> Bool;
    pred

Init ==
    /\ flag = FALSE
    /\ n = 0
    /\ pred = [i \in Int |-> i = 0]

Next ==
    /\ flag' = ~flag
    /\ n' = IF flag THEN 1 ELSE 0
    /\ pred' = [pred EXCEPT ![n] = flag]

Inv ==
    /\ (pred[n] <=> flag)
    /\ \A q \in BOOLEAN: q \/ ~q
    /\ \E i \in Int: pred[i]

=============================================================================
