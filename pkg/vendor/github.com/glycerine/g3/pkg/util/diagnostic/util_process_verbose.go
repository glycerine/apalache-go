package diagnostic

// Source: src/util/util.cpp:22-40; src/util/util.h:176-186.

import (
	"math"
	"os"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// The source verbosity level and stream pointer are process globals. Their Go
// owner is confined to one caller lane, as with the retained process services.
// The default formatter borrows os.Stderr for the lifetime of the process.
var processVerboseContext = newProcessVerboseContext()

// BorrowProcessVerboseContext supplies the one source global owner to retained
// environment-parameter and diagnostic consumers. Callers must not close it.
func BorrowProcessVerboseContext() *VerboseContext { return processVerboseContext }

func newProcessVerboseContext() *VerboseContext {
	b := arena.NewBudget(math.MaxUint64)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b, BorrowedFile: os.Stderr})
	if err != nil {
		panic(err)
	}
	c, err := NewVerboseContext(stream, b)
	if err != nil {
		panic(err)
	}
	return c
}

func SetProcessVerbosityLevel(level uint32) error {
	return processVerboseContext.SetVerbosityLevel(level)
}

func ProcessVerbosityLevel() (uint32, error) {
	return processVerboseContext.VerbosityLevel()
}

// BorrowProcessVerboseStream returns the currently selected stream pointer.
// The caller must keep any replacement stream alive while it is selected.
func BorrowProcessVerboseStream() (*utext.NumericTextStream, error) {
	return processVerboseContext.VerboseStream()
}

func SetProcessVerboseStream(stream *utext.NumericTextStream) error {
	return processVerboseContext.SetVerboseStream(stream)
}

// ProcessVerboseEnabled is the retained IF_VERBOSE level comparison.
func ProcessVerboseEnabled(level uint32) (bool, error) {
	current, err := ProcessVerbosityLevel()
	return current >= level, err
}
