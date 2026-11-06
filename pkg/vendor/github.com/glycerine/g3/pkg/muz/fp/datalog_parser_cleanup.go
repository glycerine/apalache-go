package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// datalogParserCleanupFailure keeps argument references reachable when parser
// cleanup is refused, so callers can retry releasing them.
type datalogParserCleanupFailure struct {
	parser     *datalogParser
	cause      error
	cleanupErr error
}

var _ memory.ConstructionFailure = (*datalogParserCleanupFailure)(nil)

func (f *datalogParserCleanupFailure) Error() string {
	if err := errors.Join(f.cause, f.cleanupErr); err != nil {
		return err.Error()
	}
	return "muz/fp: Datalog parser cleanup remains pending"
}

func (f *datalogParserCleanupFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanupErr)
}

func (f *datalogParserCleanupFailure) Cleanup() error {
	if f == nil || f.parser == nil {
		return nil
	}
	f.cleanupErr = f.parser.closeArguments()
	if len(f.parser.argumentRefs) == 0 {
		f.parser = nil
	}
	return f.cleanupErr
}

func (f *datalogParserCleanupFailure) RuntimeOwner() *memory.Runtime {
	if f == nil || f.parser == nil || f.parser.manager == nil || f.parser.manager.NumeralService() == nil {
		return nil
	}
	return f.parser.manager.NumeralService().RuntimeOwner()
}

func (f *datalogParserCleanupFailure) OriginalCause() error { return f.cause }
func (f *datalogParserCleanupFailure) CleanupCause() error  { return f.cleanupErr }
