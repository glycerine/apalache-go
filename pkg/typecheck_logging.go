package apalache

import "fmt"

type TypeCheckerLogger interface {
	Error(string)
	Debug(string)
}

type TypeCheckerLoggerFunc func(level, message string)

func (logger TypeCheckerLoggerFunc) Error(message string) { logger("error", message) }
func (logger TypeCheckerLoggerFunc) Debug(message string) { logger("debug", message) }

type LoggingTypeCheckerListener struct {
	sources             *SourceStore
	logger              TypeCheckerLogger
	polymorphismEnabled bool
}

func NewLoggingTypeCheckerListener(sources *SourceStore, logger TypeCheckerLogger, polymorphismEnabled bool) *LoggingTypeCheckerListener {
	if logger == nil {
		logger = TypeCheckerLoggerFunc(func(string, string) {})
	}
	return &LoggingTypeCheckerListener{sources: sources, logger: logger, polymorphismEnabled: polymorphismEnabled}
}

func (listener *LoggingTypeCheckerListener) OnTypeFound(ref ExactRef, typ Type) {
	if !listener.polymorphismEnabled && len(typ.UsedNames()) > 0 {
		message := "Found a polymorphic type: " + typ.String()
		listener.logger.Error(message)
		listener.logger.Error("Probable causes: an empty set { } needs a type annotation or an incorrect record field is used")
		panic(NewTypingInputException(message, ref.ID()))
	}
}

func (listener *LoggingTypeCheckerListener) OnTypeError(ref EtcRef, message string) {
	listener.logger.Error(fmt.Sprintf("[%s]: %s", listener.location(ref.ID()), message))
}

func (listener *LoggingTypeCheckerListener) OnTypeWarning(ref EtcRef, message string) {
	listener.logger.Debug(fmt.Sprintf("[%s]: %s", listener.location(ref.ID()), message))
}

func (listener *LoggingTypeCheckerListener) location(id UID) string {
	if listener.sources != nil {
		if location, ok := listener.sources.Find(id); ok {
			return location.String()
		}
	}
	return "unknown location"
}
