package apalache

// ExitCode is an Apalache process outcome compatible with Scala and TLC.
type ExitCode int

const (
	ExitCodeOK             ExitCode = 0
	ExitCodeCounterexample ExitCode = 12
	ExitCodeSpecEvaluation ExitCode = 75
	ExitCodeTypecheck      ExitCode = 120
	ExitCodeSpecParse      ExitCode = 150
	ExitCodeError          ExitCode = 255
)
