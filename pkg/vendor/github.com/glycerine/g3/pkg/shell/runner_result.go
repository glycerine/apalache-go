package shell

// Source: src/shell/main.cpp selected action/frontend state.

type RunnerResult struct {
	Completed bool
	Action    CommandAction
	Kind      InputKind
	InputName string
	Error     error
}

func (r *Runner) Result() (RunnerResult, error) {
	if r == nil || r.closed {
		return RunnerResult{}, ErrShellOptionService
	}
	return r.result, nil
}
