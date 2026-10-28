package apalache

// JSONDeserializationError identifies malformed imported JSON while retaining
// the underlying parse/validation cause.
type JSONDeserializationError struct {
	message string
	cause   error
}

func NewJSONDeserializationError(message string, cause error) *JSONDeserializationError {
	return &JSONDeserializationError{message: message, cause: cause}
}

func (err *JSONDeserializationError) Error() string { return err.message }
func (err *JSONDeserializationError) Unwrap() error { return err.cause }

// CurrentJSONVersion is independent of the Apalache application version.
func CurrentJSONVersion() string { return ApalacheIRJSONVersion }

// TypeTagPrinter defines the stable external string form of a type tag.
type TypeTagPrinter interface {
	PrintTypeTag(TypeTag) string
}

// TypeTagReader recovers a type tag from an external string form.
type TypeTagReader interface {
	ReadTypeTag(string) (TypeTag, error)
}

// TLAType1TagPrinter emits bare TLA type text for typed tags and the literal
// Untyped for absent tags, matching the Scala JSON representation.
type TLAType1TagPrinter struct{}

func (TLAType1TagPrinter) PrintTypeTag(tag TypeTag) string {
	if typ, ok := tag.Type(); ok {
		return typ.String()
	}
	return "Untyped"
}

// ReadTLAType1Tag is the inverse of TLAType1TagPrinter for valid Type System 1
// tags.
func ReadTLAType1Tag(text string) (TypeTag, error) {
	if text == "Untyped" {
		return Untyped(), nil
	}
	typ, err := ParseType(text)
	if err != nil {
		return Untyped(), NewJSONDeserializationError(
			"Error in type annotation: Expected Type1 expression or 'Untyped', found: "+text, err)
	}
	return Typed(typ), nil
}

// UntypedTagReader deliberately erases every serialized type annotation.
type UntypedTagReader struct{}

func (UntypedTagReader) ReadTypeTag(string) (TypeTag, error) { return Untyped(), nil }
