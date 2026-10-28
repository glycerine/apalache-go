--------------------------- MODULE StructuredTrace ---------------------------
EXTENDS Integers, Sequences, FiniteSets

VARIABLES
    \* @type: Seq(Int);
    seq,
    \* @type: Set(Int);
    set,
    \* @type: Int -> Int;
    fun

Init ==
    /\ seq = <<1, 2>>
    /\ set = {1, 2}
    /\ fun = [i \in {1, 2} |-> i]

Next ==
    /\ seq' = Append(seq, 3)
    /\ set' = set \union {3}
    /\ fun' = [fun EXCEPT ![1] = 4]

Inv == Len(seq) < 3

=============================================================================
