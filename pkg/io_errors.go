package apalache

// PrettyPrinterError classifies failures that indicate an internal rendering
// bug rather than invalid user configuration.
type PrettyPrinterError struct{ Message string }

func (err *PrettyPrinterError) Error() string {
	if err == nil {
		return ""
	}
	return err.Message
}
