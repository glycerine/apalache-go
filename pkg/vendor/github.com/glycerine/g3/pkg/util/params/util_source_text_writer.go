package params

import "io"

// sourceTextBuffer is the byte/character insertion surface used by retained
// parameter displays. bytes.Buffer and sourceTextWriter both implement it.
type sourceTextBuffer interface {
	io.Writer
	WriteByte(byte) error
	WriteString(string) (int, error)
}

// sourceTextWriter gives an io.Writer the source ostream's sticky failbit:
// insertion stops after the first failed or short write, while later source
// operations continue and can still report their own errors.
type sourceTextWriter struct {
	dst     io.Writer
	failure error
}

func (w *sourceTextWriter) Write(p []byte) (int, error) {
	if w.failure == nil && len(p) != 0 {
		n, err := w.dst.Write(p)
		if err != nil {
			w.failure = err
		} else if n != len(p) {
			w.failure = io.ErrShortWrite
		}
	}
	return len(p), nil
}
func (w *sourceTextWriter) WriteString(s string) (int, error) {
	if w.failure == nil && len(s) != 0 {
		n, err := io.WriteString(w.dst, s)
		if err != nil {
			w.failure = err
		} else if n != len(s) {
			w.failure = io.ErrShortWrite
		}
	}
	return len(s), nil
}
func (w *sourceTextWriter) WriteByte(b byte) error {
	_, _ = w.Write([]byte{b})
	return nil
}
