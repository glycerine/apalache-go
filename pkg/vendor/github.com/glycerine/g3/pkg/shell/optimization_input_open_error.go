package shell

import "fmt"

// OptimizationInputOpenError preserves opt_frontend.cpp's file diagnostic.
type OptimizationInputOpenError struct {
	Filename string
	Cause    error
}

func (e *OptimizationInputOpenError) Error() string {
	return fmt.Sprintf("(error \"failed to open file '%s'\")", e.Filename)
}

func (e *OptimizationInputOpenError) Unwrap() error { return e.Cause }
