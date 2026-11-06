package ast

// TraceFilename borrows the caller's CString without acquiring or copying it.
type TraceFilename interface {
	Byte(uint64) (byte, error)
	Length() (uint64, error)
}

func NewSourceASTManagerBorrowed(c ASTManagerConfig, filename TraceFilename) (*ASTManager, error) {
	return constructSourceASTManager(c, false, filename, nil)
}
