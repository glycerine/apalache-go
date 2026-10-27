-------------------------- MODULE RowRecordTrace -----------------------------
EXTENDS Integers

VARIABLE
\* @type: { a: Int, b: Bool };
r

Init == r = [a |-> 0, b |-> FALSE]

Next == r' = [a |-> r.a + 1, b |-> ~r.b]

Inv == r.a < 2

=============================================================================
