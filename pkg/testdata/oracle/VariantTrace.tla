----------------------------- MODULE VariantTrace ----------------------------
EXTENDS Integers, Variants

VARIABLE
    \* @type: A(Int) | B(Str);
    value

Init == value = Variant("A", 1)

Next == value' = Variant("B", "done")

Inv == VariantTag(value) = "A"

=============================================================================
