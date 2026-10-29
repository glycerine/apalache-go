-------------------- MODULE VariableNonlinearDeadlock --------------------
EXTENDS Integers

VARIABLE
    \* @type: Int;
    x

Init == \E i \in {2, 4}: x = i

Next == x' = 0 /\ \E j \in 0..x: j * j = x

=============================================================================
