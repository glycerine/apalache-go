package shell

import (
	"fmt"
	"os"
	"time"
)

func (f *DatalogFrontend) Load(path string) error {
	if f == nil || f.context == nil || f.loaded || path == "" {
		return ErrDatalogFrontend
	}
	f.started, f.path = time.Now(), path
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	// read_datalog constructs its context with a local Datalog engine
	// choice, overriding a conflicting fixedpoint module engine parameter.
	if err = f.context.SelectDatalogFrontendEngine(); err != nil {
		return err
	}
	if info.IsDir() {
		err = f.context.ParseWPADirectory(path)
	} else {
		err = f.context.ParseDatalogFile(path)
	}
	f.parseFinished = time.Now()
	if err != nil {
		return fmt.Errorf("Datalog parse %s: %w", path, err)
	}
	f.loaded = true
	return nil
}
