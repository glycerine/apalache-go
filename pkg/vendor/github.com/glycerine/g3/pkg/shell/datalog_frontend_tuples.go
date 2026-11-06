package shell

import "io"

// WriteTuples renders source output-pragmas from the completed saturation.
// A refused writer can be retried without rerunning or mutating the query.
func (f *DatalogFrontend) WriteTuples(out io.Writer) error {
	if f == nil || !f.saturated || out == nil {
		return ErrDatalogFrontend
	}
	if !f.printTuples {
		return nil
	}
	return f.context.WriteFixedpointDatalogTuples(out)
}
