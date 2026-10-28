package apalache

import "errors"

// PassExecError reports an error while executing a pass sequence.
type PassExecError struct{ Text string }

func (err *PassExecError) Error() string {
	if err == nil {
		return "<nil pass execution error>"
	}
	return err.Text
}

// PassOptionError reports invalid pass configuration.
type PassOptionError struct{ Text string }

func (err *PassOptionError) Error() string {
	if err == nil {
		return "<nil pass option error>"
	}
	return err.Text
}

// ErrorMessage classifies an adapted error for user-facing reporting.
type ErrorMessage interface {
	Message() string
	RequiresStackTrace() bool
}

type NormalErrorMessage struct{ Text string }

func (message NormalErrorMessage) Message() string          { return message.Text }
func (message NormalErrorMessage) RequiresStackTrace() bool { return false }

type FailureMessage struct{ Text string }

func (message FailureMessage) Message() string          { return message.Text }
func (message FailureMessage) RequiresStackTrace() bool { return true }

// ExceptionAdapter converts errors recognized by a command or subsystem into
// stable user-facing messages. matched is false for errors outside its domain.
type ExceptionAdapter interface {
	Adapt(error) (message ErrorMessage, matched bool)
}

type ExceptionAdapterFunc func(error) (ErrorMessage, bool)

func (adapt ExceptionAdapterFunc) Adapt(err error) (ErrorMessage, bool) {
	return adapt(err)
}

// DefaultExceptionAdapter has no Go-specific mappings. Unlike a JVM process,
// a Go process cannot reliably recover a runtime out-of-memory fatal error.
type DefaultExceptionAdapter struct{}

func (DefaultExceptionAdapter) Adapt(error) (ErrorMessage, bool) { return nil, false }

// EtcTypeCheckerAdapter classifies the user and internal failures emitted by
// ETC translation, inference, and type rewriting.
type EtcTypeCheckerAdapter struct{ Sources *SourceStore }

func (adapter EtcTypeCheckerAdapter) Adapt(err error) (ErrorMessage, bool) {
	var input *TypingInputException
	if errors.As(err, &input) {
		return NormalErrorMessage{Text: "Typing input error: " + input.Error()}, true
	}
	var outdated *OutdatedAnnotationsError
	if errors.As(err, &outdated) {
		location := "<unknown>"
		if adapter.Sources != nil {
			if found, ok := adapter.Sources.Find(outdated.CauseExpr().ID()); ok {
				location = found.String()
			}
		}
		return NormalErrorMessage{Text: location + ": rewriter error: " + outdated.Error()}, true
	}
	var typing *TypingException
	if errors.As(err, &typing) {
		return FailureMessage{Text: "Type checker error: " + typing.Error()}, true
	}
	return nil, false
}

// AdaptedError retains the message classification produced by an adapter.
type AdaptedError struct{ Message ErrorMessage }

func (err *AdaptedError) Error() string {
	if err == nil || err.Message == nil {
		return "<nil adapted error>"
	}
	return err.Message.Message()
}
