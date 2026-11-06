package api

import (
	"bytes"

	"github.com/glycerine/g3/pkg/shell"
)

func (c *SMTContext) ProfileDatalogPath(path string) (result DatalogProfileResult, err error) {
	if err = c.open(); err != nil {
		return result, err
	}
	var timings, profile bytes.Buffer
	frontend, err := shell.NewDatalogFrontend(c.command, &timings)
	if err != nil {
		return result, err
	}
	if err = frontend.Run(path); err != nil {
		result.Output = timings.String()
		return result, err
	}
	result.Profile, err = frontend.Profile()
	if err != nil {
		return result, err
	}
	result.Timings, err = frontend.Timings()
	if err == nil {
		err = frontend.WriteProfile(&profile)
	}
	result.Output = timings.String() + profile.String()
	return result, err
}
