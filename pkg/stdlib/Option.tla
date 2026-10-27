------------------- MODULE Option -----------------------

EXTENDS Apalache, Variants

\* @typeAlias: option = Some(a) | None(UNIT);

\* @type: a => $option;
Some(__x) == Variant("Some", __x)

\* @type: () => $option;
None == Variant("None", UNIT)

\* @type: $option => Bool;
IsSome(__o) == VariantTag(__o) = "Some"

\* @type: $option => Bool;
IsNone(__o) == VariantTag(__o) = "None"

\* @type: (Some(a) | None(UNIT), a => b, UNIT => b) => b;
OptionCase(__o, __caseSome(_), __caseNone(_)) ==
  IF IsSome(__o)
  THEN __caseSome(VariantGetUnsafe("Some", __o))
  ELSE __caseNone(UNIT)

\* @type: (a => b, Some(a) | None(UNIT)) => Some(b) | None(UNIT);
OptionMap(__f(_), __o) ==
  IF IsSome(__o)
  THEN Some(__f(VariantGetUnsafe("Some", __o)))
  ELSE None

\* @type: (a => Some(b) | None(UNIT), Some(a) | None(UNIT)) => Some(b) | None(UNIT);
OptionFlatMap(__f(_), __o) ==
  IF IsSome(__o)
  THEN __f(VariantGetUnsafe("Some", __o))
  ELSE None

\* @type: ($option, a) => a;
OptionGetOrElse(__o, __default) ==
  IF IsSome(__o)
  THEN VariantGetUnsafe("Some", __o)
  ELSE __default

\* @type: (Some(a) | None(UNIT)) => Seq(a);
OptionToSeq(__o) ==
  IF IsSome(__o)
  THEN <<VariantGetUnsafe("Some", __o)>>
  ELSE <<>>

\* @type: (Some(a) | None(UNIT)) => Set(a);
OptionToSet(__o) ==
  IF IsSome(__o)
  THEN {VariantGetUnsafe("Some", __o)}
  ELSE {}

\* @type: Set(a) => Some(a) | None(UNIT);
OptionGuess(__s) ==
  LET __getter(__oa, __b) == IF IsSome(__oa) THEN __oa ELSE Some(__b) IN
  ApaFoldSet(__getter, None, __s)

\* @type: (a -> b, Some(a) | None(UNIT)) => Some(b) | None(UNIT);
OptionFunApp(__f, __o) ==
  IF IsSome(__o)
  THEN Some(__f[VariantGetUnsafe("Some", __o)])
  ELSE None

\* @type: (a -> b, Set(a)) => (a -> Some(b) | None(UNIT));
OptionPartialFun(__f, __undefined) ==
  [__x \in DOMAIN __f \union __undefined |->
    IF __x \in __undefined THEN None ELSE Some(__f[__x])]

============================================================
