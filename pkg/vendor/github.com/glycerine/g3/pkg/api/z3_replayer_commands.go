package api

func (r *Z3Replayer) RegisterCommand(id uint32, name string, command ReplayCommand) error {
	if r == nil || r.closed || command == nil {
		return ErrZ3ReplayState
	}
	r.commands[id] = replayCommandEntry{name: name, call: command}
	return nil
}

func (r *Z3Replayer) StoreResult(object uint64) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	r.result = object
	return nil
}

func (r *Z3Replayer) resetArguments() {
	r.result = 0
	r.args = nil
	r.objectArrays = nil
	r.symbolArrays = nil
	r.uintArrays = nil
	r.intArrays = nil
}
