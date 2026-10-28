package apalache

// LirError is the root of errors originating in the logical intermediate
// representation. Concrete LIR errors unwrap to this value so callers can use
// errors.As to recognize the same hierarchy as Scala.
type LirError struct{ message string }

func NewLirError(message string) *LirError { return &LirError{message: message} }
func (err *LirError) Error() string        { return err.message }

// LanguagePredError reports nodes that do not belong to an expected language
// fragment.
type LanguagePredError struct {
	*LirError
	failedIDs []LanguageFailure
}

func NewLanguagePredError(message string, failedIDs []LanguageFailure) *LanguagePredError {
	return &LanguagePredError{LirError: NewLirError(message), failedIDs: append([]LanguageFailure(nil), failedIDs...)}
}
func (err *LanguagePredError) FailedIDs() []LanguageFailure {
	return append([]LanguageFailure(nil), err.failedIDs...)
}
func (err *LanguagePredError) Unwrap() error { return err.LirError }

// MalformedTlaError identifies the malformed expression that caused a
// structural IR failure.
type MalformedTlaError struct {
	*LirError
	causeExpr Expr
}

func NewMalformedTlaError(message string, causeExpr Expr) *MalformedTlaError {
	return &MalformedTlaError{LirError: NewLirError(message), causeExpr: causeExpr}
}
func (err *MalformedTlaError) CauseExpr() Expr { return err.causeExpr }
func (err *MalformedTlaError) Unwrap() error   { return err.LirError }

// MissingTransformationError identifies a module used before its required
// transformations were applied.
type MissingTransformationError struct {
	*LirError
	causeModule *Module
}

func NewMissingTransformationError(message string, causeModule *Module) *MissingTransformationError {
	return &MissingTransformationError{LirError: NewLirError(message), causeModule: causeModule}
}
func (err *MissingTransformationError) CauseModule() *Module { return err.causeModule }
func (err *MissingTransformationError) Unwrap() error        { return err.LirError }

// MalformedSepecificationError retains the misspelling in the Scala API. It
// intentionally does not inherit from LirError.
type MalformedSepecificationError struct{ message string }

func NewMalformedSepecificationError(message string) *MalformedSepecificationError {
	return &MalformedSepecificationError{message: message}
}
func (err *MalformedSepecificationError) Error() string { return err.message }

type BuilderError struct{ *LirError }

func NewBuilderError(message string) *BuilderError {
	return &BuilderError{LirError: NewLirError(message)}
}
func (err *BuilderError) Unwrap() error { return err.LirError }

// TypingException is intentionally outside the LIR-error hierarchy and may
// use NullUID when no concrete expression caused the failure.
type TypingException struct {
	message     string
	causeExprID UID
}

func NewTypingException(message string, causeExprID UID) *TypingException {
	return &TypingException{message: message, causeExprID: causeExprID}
}
func (err *TypingException) Error() string    { return err.message }
func (err *TypingException) CauseExprID() UID { return err.causeExprID }

type OutdatedAnnotationsError struct {
	*LirError
	causeExpr Expr
}

func NewOutdatedAnnotationsError(message string, causeExpr Expr) *OutdatedAnnotationsError {
	return &OutdatedAnnotationsError{LirError: NewLirError(message), causeExpr: causeExpr}
}
func (err *OutdatedAnnotationsError) CauseExpr() Expr { return err.causeExpr }
func (err *OutdatedAnnotationsError) Unwrap() error   { return err.LirError }

// UnsupportedFeatureError is a plain exception in Scala, not a LirError.
type UnsupportedFeatureError struct{ message string }

func NewUnsupportedFeatureError(message string) *UnsupportedFeatureError {
	return &UnsupportedFeatureError{message: message}
}
func (err *UnsupportedFeatureError) Error() string { return err.message }
