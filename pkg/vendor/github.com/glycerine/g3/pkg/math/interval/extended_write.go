package interval

import "io"

// writeExtendedText keeps the completed source-order insertion prefix and
// reports a short Go writer result to the owning interval display.
func writeExtendedText(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}
