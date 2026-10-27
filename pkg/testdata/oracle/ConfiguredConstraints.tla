---- MODULE ConfiguredConstraints ----

EXTENDS Integers

VARIABLE
    \* @type: Int;
    x

Init == x = 0

Next == x' = x + 1

StateOK == x < 2

ActionOK == x' < 2

Inv == TRUE

====
