package api

import "io"

func (o *APIOptimize) WriteOptimizationModelValidation(out io.Writer) (uint32, error) {
	if err := o.open(); err != nil {
		return 0, err
	}
	return o.command.WriteOptimizationModelValidation(out)
}

func (o *APIOptimize) OptimizationModelValid() (bool, error) {
	if err := o.open(); err != nil {
		return false, err
	}
	return o.command.OptimizationModelValid()
}
