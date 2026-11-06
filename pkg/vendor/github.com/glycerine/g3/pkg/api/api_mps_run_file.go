package api

import (
	"errors"
	"os"
)

func (c *SMTContext) RunMPSFile(path string, options *MPSRunOptions) (result MPSRunResult, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	return c.RunMPS(file, path, options)
}
