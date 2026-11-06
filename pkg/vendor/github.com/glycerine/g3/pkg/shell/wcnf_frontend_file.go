package shell

import (
	"io"
	"os"
)

func (f *WCNFFrontend) RunFile(path string) error {
	return f.run(func() (io.Reader, io.Closer, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, nil, &OptimizationInputOpenError{Filename: path, Cause: err}
		}
		return file, file, nil
	})
}
