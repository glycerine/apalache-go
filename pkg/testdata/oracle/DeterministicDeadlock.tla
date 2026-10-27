---- MODULE DeterministicDeadlock ----

EXTENDS Integers

VARIABLE
    \* @type: Int;
    x

Init == x = 0

Next ==
    /\ x < 2
    /\ x' = x + 1

Inv == x >= 0

====
