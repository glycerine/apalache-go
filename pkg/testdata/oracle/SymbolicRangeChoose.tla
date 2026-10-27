---------------------- MODULE SymbolicRangeChoose ----------------------------
EXTENDS Integers

VARIABLE
\* @type: Int;
x

Init == x = 0
Next == x' = x
Inv == (CHOOSE i \in x..(x + 2): i > x) > x

=============================================================================
