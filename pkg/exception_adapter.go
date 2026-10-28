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

// CheckerExceptionAdapter classifies the user-facing and internal failures
// emitted by the standalone checker pipeline. Source locations are resolved
// through transformation ancestry, matching the Scala checker adapter.
type CheckerExceptionAdapter struct {
	Sources *SourceStore
	Changes *ChangeListener
}

func (adapter CheckerExceptionAdapter) location(id UID) string {
	if adapter.Sources != nil {
		locator := NewSourceLocator(adapter.Sources.SourceMap(), adapter.Changes)
		if found, ok := locator.SourceOfID(id); ok {
			return found.String()
		}
	}
	return "<unknown>"
}

func (adapter CheckerExceptionAdapter) Adapt(err error) (ErrorMessage, bool) {
	var configuration *ConfigurationError
	if errors.As(err, &configuration) {
		return NormalErrorMessage{Text: "Configuration error (see the manual): " + configuration.Error()}, true
	}
	var override *OverrideError
	if errors.As(err, &override) {
		return NormalErrorMessage{Text: "Configuration error (see the manual): " + override.Error()}, true
	}
	var assignment *AssignmentError
	if errors.As(err, &assignment) {
		return NormalErrorMessage{Text: "Assignment error: " + assignment.Error()}, true
	}
	var outdated *OutdatedAnnotationsError
	if errors.As(err, &outdated) {
		return NormalErrorMessage{Text: adapter.location(outdated.CauseExpr().ID()) + ": rewriter error: " + outdated.Error()}, true
	}
	var language *LanguagePredError
	if errors.As(err, &language) {
		return NormalErrorMessage{Text: "Unexpected expressions in the specification (see the error messages)"}, true
	}
	var input *TypingInputException
	if errors.As(err, &input) {
		return NormalErrorMessage{Text: adapter.location(input.CauseExprID()) + ": type input error: " + input.Error()}, true
	}
	var typing *TypingException
	if errors.As(err, &typing) {
		return FailureMessage{Text: adapter.location(typing.CauseExprID()) + ": internal error in type checking: " + typing.Error()}, true
	}
	var jsonError *JSONDeserializationError
	if errors.As(err, &jsonError) {
		return NormalErrorMessage{Text: "Error in JSON deserialization. Perhaps the file is malformed?: " + jsonError.Error() + "."}, true
	}
	var itfError *ITFError
	if errors.As(err, &itfError) {
		return NormalErrorMessage{Text: "Error in ITF deserialization. Perhaps the file is malformed?: " + itfError.Error() + "."}, true
	}
	var malformed *MalformedTlaError
	if errors.As(err, &malformed) {
		return FailureMessage{Text: adapter.location(malformed.CauseExpr().ID()) + ": unexpected TLA+ expression: " + malformed.Error()}, true
	}
	var noRule *NoRuleException
	if errors.As(err, &noRule) {
		return FailureMessage{Text: adapter.location(noRule.CauseExpr().ID()) + ": no rule to rewrite a TLA+ expression: " + noRule.Error()}, true
	}
	var limitation *RewriterKnownLimitationError
	if errors.As(err, &limitation) {
		return NormalErrorMessage{Text: adapter.location(limitation.CauseExpr().ID()) + ": known limitation: " + limitation.Error()}, true
	}
	var rewriter *RewriterException
	if errors.As(err, &rewriter) {
		return FailureMessage{Text: adapter.location(rewriter.CauseExpr().ID()) + ": rewriter error: " + rewriter.Error()}, true
	}
	var encoding *SmtEncodingException
	if errors.As(err, &encoding) {
		return FailureMessage{Text: adapter.location(encoding.CauseExpr().ID()) + ": error when rewriting to SMT: " + encoding.Error()}, true
	}
	var invalid *InvalidTlaExException
	if errors.As(err, &invalid) {
		return FailureMessage{Text: adapter.location(invalid.CauseExpr().ID()) + ": unexpected TLA+ expression: " + invalid.Error()}, true
	}
	var internal *InternalCheckerError
	if errors.As(err, &internal) {
		return FailureMessage{Text: adapter.location(internal.CauseExpr().ID()) + ": internal error: " + internal.Error()}, true
	}
	var specification *MalformedSepecificationError
	if errors.As(err, &specification) {
		return NormalErrorMessage{Text: "The specification is malformed: " + specification.Error()}, true
	}
	var checker *CheckerException
	if errors.As(err, &checker) {
		return FailureMessage{Text: adapter.location(checker.CauseExpr().ID()) + ": checker error: " + checker.Error()}, true
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
