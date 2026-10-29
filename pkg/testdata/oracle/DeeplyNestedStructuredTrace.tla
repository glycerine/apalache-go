---------------------- MODULE DeeplyNestedStructuredTrace ----------------------
EXTENDS Integers, Sequences, Variants

VARIABLE
    \* @type: { history: Seq({ nums: Set(Int), tag: A(<<Int, Bool>>) | B(Int -> Int) }), lookup: Int -> { vals: Seq(Int), flags: Set(Bool) } };
    state

Inner(i) ==
    [ nums |-> {i, i + 1},
      tag  |-> Variant("A", <<i, i = 0>>) ]

\* @type: Int => (Int -> { vals: Seq(Int), flags: Set(Bool) });
Lookup(i) ==
    [k \in {1, 2} |->
        [ vals  |-> <<i, k>>,
          flags |-> {i = 0, k = 1} ]]

Init ==
    state =
        [ history |-> <<Inner(0)>>,
          lookup  |-> Lookup(0) ]

Next ==
    state' =
        [ history |-> Append(state.history, Inner(1)),
          lookup  |-> Lookup(1) ]

Inv == Len(state.history) < 2

===============================================================================
