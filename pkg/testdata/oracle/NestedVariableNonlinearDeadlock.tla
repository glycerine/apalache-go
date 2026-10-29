---------------- MODULE NestedVariableNonlinearDeadlock ----------------
EXTENDS Integers

VARIABLE
    \* @type: Int;
    x

Init == \E n \in {2, 4}: x = n

Next == x' = 0 /\ \E i \in 0..x: \E j \in 0..i: j * j = x

=============================================================================
