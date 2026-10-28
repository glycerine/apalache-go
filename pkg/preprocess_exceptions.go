package apalache

type NotInKeraError struct{ *MalformedTlaError }

func NewNotInKeraError(message string, cause Expr) *NotInKeraError {
	return &NotInKeraError{NewMalformedTlaError(message, cause)}
}
func (err *NotInKeraError) Unwrap() error { return err.MalformedTlaError }

type OverridingError struct{ *MalformedTlaError }

func NewOverridingError(message string, cause Expr) *OverridingError {
	return &OverridingError{NewMalformedTlaError(message, cause)}
}
func (err *OverridingError) Unwrap() error { return err.MalformedTlaError }

type IrrecoverablePreprocessingError struct{ Message string }

func (err *IrrecoverablePreprocessingError) Error() string { return err.Message }

type TlaInputError struct {
	Message  string
	SourceID *UID
}

func (err *TlaInputError) Error() string { return err.Message }

func NewTlaInputError(message string, sourceID ...UID) *TlaInputError {
	err := &TlaInputError{Message: message}
	if len(sourceID) > 0 {
		id := sourceID[0]
		err.SourceID = &id
	}
	return err
}

type QuintUnsupportedError struct{ Message string }

func NewQuintUnsupportedError(message string) *QuintUnsupportedError {
	return &QuintUnsupportedError{Message: "Unsupported quint input: " + message}
}
func (err *QuintUnsupportedError) Error() string { return err.Message }
