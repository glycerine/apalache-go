package api

func (r *Z3Replayer) saveResult(pointer uint64) { r.heap[pointer] = r.result }

func (r *Z3Replayer) pushObject(pointer uint64) error {
	if pointer == 0 {
		r.args = append(r.args, z3ReplayValue{kind: replayObject})
		return nil
	}
	object, found := r.heap[pointer]
	if !found {
		return r.failure("invalid pointer")
	}
	r.args = append(r.args, z3ReplayValue{kind: replayObject, u: object})
	return nil
}

func (r *Z3Replayer) saveArgument(pointer, position uint64) error {
	value, err := r.value(uint32(position), replayObject)
	if err != nil {
		return err
	}
	r.heap[pointer] = value.u
	return nil
}

func (r *Z3Replayer) saveArrayElement(pointer, position, index uint64) error {
	values, err := r.ObjectArray(uint32(position))
	if err != nil {
		return err
	}
	if index >= uint64(len(values)) {
		return ErrZ3ReplayReference
	}
	r.heap[pointer] = values[index]
	return nil
}
