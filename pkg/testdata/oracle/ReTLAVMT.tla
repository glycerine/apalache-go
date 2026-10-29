------------------------------ MODULE ReTLAVMT ------------------------------

CONSTANT
    \* @type: Set(A);
    S

VARIABLE
    \* @type: A -> A;
    f

Init == f = [x \in S |-> "a_OF_A"]

Next == f' = [f EXCEPT !["a_OF_A"] = "b_OF_A"]

Inv == f["a_OF_A"] = "a_OF_A" \/ f["a_OF_A"] = "b_OF_A"

=============================================================================
