-------------------- MODULE NondeterministicStructuredTrace --------------------
EXTENDS Integers, Sequences, Variants

VARIABLES
    \* @type: Seq(Int);
    seq,
    \* @type: Set(Int);
    set,
    \* @type: Int -> Int;
    fun,
    \* @type: { a: Int, b: Bool };
    rec,
    \* @type: A(Int) | B(Int);
    variant

Init ==
    \E i \in {0, 1}:
        /\ seq = <<i>>
        /\ set = {i}
        /\ fun = [j \in {1, 2} |-> j + i]
        /\ rec = [a |-> i, b |-> i = 0]
        /\ variant = Variant("A", i)

Next ==
    \E i \in {3, 4}:
        /\ seq' = Append(seq, i)
        /\ set' = set \union {i}
        /\ fun' = [fun EXCEPT ![1] = i]
        /\ rec' = [a |-> i, b |-> ~rec.b]
        /\ variant' = Variant("B", i)

Inv == Len(seq) < 2

=============================================================================
