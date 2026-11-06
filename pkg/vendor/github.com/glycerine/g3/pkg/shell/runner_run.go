package shell

// Source: src/shell/main.cpp one-shot executable composition.

import "errors"

func RunShell(c RunnerConfig) (result error) {
	runner, err := NewRunner(c)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, runner.Close()) }()
	return runner.Run()
}
