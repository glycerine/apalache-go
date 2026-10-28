---------------------------- MODULE TLC ---------------------------------------
LOCAL INSTANCE Naturals

\* @type: (a, b) => a -> b;
key :> value == [x \in {key} |-> value]

\* @type: (a -> b, a -> b) => a -> b;
left @@ right ==
  [x \in DOMAIN left \union DOMAIN right |->
    IF x \in DOMAIN left THEN left[x] ELSE right[x]]

\* @type: (a, b) => b;
Print(output, value) == value

\* @type: a => Bool;
PrintT(output) == TRUE

\* @type: (Bool, Str) => Bool;
Assert(condition, output) == condition

\* @type: () => Int;
JavaTime == 123

\* @type: () => Set(a);
__TLCEmptySet == {}

\* @type: b => a;
TLCGet(key) == CHOOSE value \in __TLCEmptySet: TRUE

\* @type: (b, a) => Bool;
TLCSet(key, value) == TRUE

\* @type: Set(a) => Set(a -> a);
Permutations(S) ==
  {f \in [S -> S] : \A w \in S : \E v \in S : f[v] = w}

\* TLC returns a function with the same domain as the input sequence.
\* @type: (Seq(a), (a, a) => Bool) => (Int -> a);
SortSeq(s, LessThan(_, _)) ==
  LET permutation == CHOOSE p \in Permutations(DOMAIN s):
        \A i, j \in DOMAIN s:
          \/ (i < j) => LessThan(s[p[i]], s[p[j]])
          \/ s[p[i]] = s[p[j]]
  IN [i \in DOMAIN s |-> s[permutation[i]]]

\* @type: () => a;
Any ==
  LET \* @type: () => Set(a);
      empty == {}
  IN CHOOSE value \in empty: TRUE

\* @type: Set(a) => a;
RandomElement(S) == CHOOSE x \in S: TRUE

\* @type: a => Str;
ToString(value) == ""

\* @type: a => a;
TLCEval(value) == value
===============================================================================
