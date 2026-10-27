------------------------ MODULE SequencesExt ---------------------------------
EXTENDS Integers, Sequences, FiniteSets, Apalache, __apalache_internal

\* @type: Seq(a) => Set(a);
ToSet(s) == {s[i]: i \in DOMAIN s}

\* @type: (Seq(a), a) => Seq(a);
__AppendToSeq(seq, element) == Append(seq, element)

\* @type: Set(a) => Seq(a);
SetToSeq(S) ==
  LET Add(seq, element) == __AppendToSeq(seq, element)
  IN ApaFoldSet(Add, <<>>, S)

\* @type: ((a, a) => Bool, Seq(a), a) => Seq(a);
__InsertSorted(LessThan(_, _), seq, newElement) ==
  LET nextLen == Len(seq) + 1 IN
  LET position ==
    CHOOSE p \in DOMAIN seq \union {nextLen}:
      \A i \in DOMAIN seq: i < p <=> LessThan(seq[i], newElement)
  IN
  LET CopyOrSet(i) ==
    IF i < position THEN seq[i]
    ELSE IF i = position THEN newElement ELSE seq[i - 1]
  IN SubSeq(MkSeq(__ApalacheSeqCapacity(seq) + 1, CopyOrSet), 1, nextLen)

\* @type: (Set(a), (a, a) => Bool) => Seq(a);
SetToSortSeq(S, LessThan(_, _)) ==
  LET InsertSorted(seq, newElement) == __InsertSorted(LessThan, seq, newElement)
  IN ApaFoldSet(InsertSorted, <<>>, S)

\* @type: (Seq(a), a) => Bool;
Contains(s, element) == \E i \in DOMAIN s: s[i] = element

\* @type: Seq(a) => Seq(a);
Reverse(s) ==
  LET length == Len(s) IN
  LET Get(i) == s[length - i + 1] IN
  SubSeq(MkSeq(__ApalacheSeqCapacity(s), Get), 1, length)

\* @type: (Seq(a), a) => Seq(a);
Remove(seq, element) ==
  LET AppendUnlessEqual(result, candidate) ==
    IF candidate /= element THEN Append(result, candidate) ELSE result
  IN ApaFoldSeqLeft(AppendUnlessEqual, <<>>, seq)

\* @type: (Seq(a), a, a) => Seq(a);
ReplaceAll(seq, old, new) ==
  LET CopyOrSet(i) == IF seq[i] = old THEN new ELSE seq[i] IN
  SubSeq(MkSeq(__ApalacheSeqCapacity(seq), CopyOrSet), 1, Len(seq))

\* @type: (Seq(a), Int, a) => Seq(a);
InsertAt(seq, k, element) ==
  LET CopyOrSet(i) ==
    IF i = k THEN element ELSE IF i < k THEN seq[i] ELSE seq[i - 1]
  IN SubSeq(MkSeq(__ApalacheSeqCapacity(seq) + 1, CopyOrSet), 1, Len(seq) + 1)

\* @type: (Seq(a), Int, a) => Seq(a);
ReplaceAt(seq, i, element) == [seq EXCEPT ![i] = element]

\* @type: (Seq(a), Int) => Seq(a);
RemoveAt(seq, i) == SubSeq(seq, 1, i - 1) \o SubSeq(seq, i + 1, Len(seq))

\* @type: (a, Seq(a)) => Seq(a);
Cons(element, seq) == <<element>> \o seq

\* @type: Seq(a) => Seq(a);
Front(seq) == SubSeq(seq, 1, Len(seq) - 1)

\* @type: Seq(a) => a;
Last(seq) == seq[Len(seq)]

\* @type: (Seq(a), Seq(a)) => Bool;
IsPrefix(s, t) == s = SubSeq(t, 1, Len(s))

\* @type: (Seq(a), Seq(a)) => Bool;
IsStrictPrefix(s, t) == IsPrefix(s, t) /\ s /= t

\* @type: (Seq(a), Seq(a)) => Bool;
IsSuffix(s, t) == IsPrefix(Reverse(s), Reverse(t))

\* @type: (Seq(a), Seq(a)) => Bool;
IsStrictSuffix(s, t) == IsSuffix(s, t) /\ s /= t

\* @type: Seq(a) => Set(Seq(a));
Prefixes(s) == {SubSeq(s, 1, length): length \in {0} \union DOMAIN s}

\* @type: Set(Seq(a)) => Set(Seq(a));
CommonPrefixes(S) ==
  LET candidates == UNION {Prefixes(seq): seq \in S}
  IN {prefix \in candidates: \A seq \in S: IsPrefix(prefix, seq)}

\* @type: Set(Seq(a)) => Seq(a);
LongestCommonPrefix(S) ==
  CHOOSE longest \in CommonPrefixes(S):
    \A other \in CommonPrefixes(S): Len(other) <= Len(longest)

\* @type: (Int, Int) => Int;
SeqMod(a, b) == IF a % b = 0 THEN b ELSE a % b

\* @type: ((a, b) => b, b, a) => b;
__FoldSeqStep(op(_, _), acc, element) == op(element, acc)

\* @type: ((a, b) => b, b, Seq(a)) => b;
FoldSeq(op(_, _), base, seq) ==
  LET Step(acc, element) == __FoldSeqStep(op, acc, element)
  IN ApaFoldSeqLeft(Step, base, seq)

\* @type: ((b, a) => b, b, a) => b;
__FoldLeftStep(op(_, _), acc, element) == op(acc, element)

\* @type: ((b, a) => b, b, Seq(a)) => b;
FoldLeft(op(_, _), base, seq) ==
  LET Step(acc, element) == __FoldLeftStep(op, acc, element)
  IN ApaFoldSeqLeft(Step, base, seq)

\* @type: ((a, b) => b, b, a) => b;
__FoldRightStep(op(_, _), acc, element) == op(element, acc)

\* @type: ((a, b) => b, Seq(a), b) => b;
FoldRight(op(_, _), seq, base) ==
  LET Step(acc, element) == __FoldRightStep(op, acc, element)
  IN ApaFoldSeqLeft(Step, base, Reverse(seq))

\* @type: (Seq(a), Seq(a)) => Seq(a);
__ConcatSeq(left, right) == left \o right

\* @type: Seq(Seq(a)) => Seq(a);
FlattenSeq(seqs) ==
  LET Concat(acc, seq) == __ConcatSeq(acc, seq)
  IN ApaFoldSeqLeft(Concat, <<>>, seqs)

\* @type: (Seq(a), Seq(b), Int) => <<a, b>>;
__ZipTuple(s, t, i) == <<s[i], t[i]>>

\* @type: (Seq(a), Seq(b)) => Seq(<<a, b>>);
Zip(s, t) ==
  LET MakeTuple(i) == __ZipTuple(s, t, i) IN
  IF Len(s) /= Len(t) THEN <<>>
  ELSE SubSeq(MkSeq(__ApalacheSeqCapacity(s), MakeTuple), 1, Len(s))

\* @type: Seq(a) => Set(Seq(a));
SubSeqs(s) ==
  {SubSeq(s, i + 1, j): i, j \in {0} \union DOMAIN s}

\* @type: (Seq(a), Seq(a)) => Int;
IndexFirstSubSeq(needle, haystack) ==
  LET needleLength == Len(needle) IN
  LET IsSubSeq(i) ==
    needle = SubSeq(haystack, i, i + needleLength - 1)
  IN
  LET domain0 == {0} \union DOMAIN haystack IN
  CHOOSE i \in domain0:
    IsSubSeq(i) /\ \A j \in domain0: j < i => ~IsSubSeq(j)

\* @type: (Int, Seq(a), Seq(a), Seq(a)) => Seq(a);
ReplaceSubSeqAt(i, replacement, old, target) ==
  LET prefix == SubSeq(target, 1, i - 1)
      suffix == SubSeq(target, i + Len(old), Len(target))
  IN prefix \o replacement \o suffix

\* @type: (Seq(a), Seq(a), Seq(a)) => Seq(a);
ReplaceFirstSubSeq(replacement, old, target) ==
  IF \E i \in {0} \union DOMAIN target:
       old = SubSeq(target, i, i + Len(old))
  THEN ReplaceSubSeqAt(IndexFirstSubSeq(old, target), replacement, old, target)
  ELSE target
===============================================================================
