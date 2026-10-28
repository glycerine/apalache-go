package apalache

// BuilderT and BuilderUT mirror the Scala package aliases for delayed scoped
// builder instructions and their built IR expressions.
type BuilderT = ExprInstruction
type BuilderUT = Expr

// TLA and TLAUnsafe are the strict premade scope-safe and scope-unsafe
// builders exposed by the Scala types package object. Both builders are
// immutable configuration holders; each scoped Build allocates fresh state.
var (
	TLA       = NewScopedBuilder()
	TLAUnsafe = NewTypedBuilder()
)
