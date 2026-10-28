package apalache

// TraceeState and TraceeStateSeq are the Go-qualified equivalents of the
// State and StateSeq aliases in Scala's tracee package object.
type TraceeState = TraceState
type TraceeStateSeq = []TraceState

// TraceeBridgingPass is the identity bridge used when trace evaluation skips
// analysis. Its Analyzed transformation metadata satisfies the BMC boundary.
type TraceeBridgingPass struct{}

func NewTraceeBridgingPass() TraceeBridgingPass { return TraceeBridgingPass{} }

func (TraceeBridgingPass) Name() string { return "BridgingPass" }
func (TraceeBridgingPass) Dependencies() []ModuleProperty {
	return nil
}
func (TraceeBridgingPass) Transformations() []ModuleProperty {
	return []ModuleProperty{PropertyAnalyzed}
}
func (TraceeBridgingPass) Execute(module *Module) (*Module, error) { return module, nil }

var _ ModulePass = TraceeBridgingPass{}
