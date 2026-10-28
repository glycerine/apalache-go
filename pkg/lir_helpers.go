package apalache

// FindBodyOf returns the body of the first declaration with the requested
// name when that declaration is an operator. This intentionally does not scan
// past a same-named non-operator declaration, matching Scala's find followed
// by withFilter. Missing and non-operator matches return the NullExpr sentinel.
func FindBodyOf(name string, declarations ...Decl) Expr {
	for _, declaration := range declarations {
		if declaration == nil || declaration.Name() != name {
			continue
		}
		if operator, ok := declaration.(*OperDecl); ok {
			return operator.Body()
		}
		return NullExpr()
	}
	return NullExpr()
}
