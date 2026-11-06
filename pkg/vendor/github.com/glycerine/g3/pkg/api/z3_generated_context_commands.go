package api

// Source: generated src/api/api_commands.cpp, context creation, destruction,
// parameter update and interruption handlers.

import "errors"

type replayContextOwner struct {
	context                *SMTContext
	userRefCount           bool
	printMode              uint32
	parserError            string
	results                *apiContextResults
	astIDs                 []uint64
	parameterIDs           []uint64
	descriptionIDs         []uint64
	solverIDs              []uint64
	modelIDs               []uint64
	goalIDs                []uint64
	vectorIDs              []uint64
	functionIDs            []uint64
	functionEntryIDs       []uint64
	tacticIDs              []uint64
	tacticResultIDs        []uint64
	probeIDs               []uint64
	statisticIDs           []uint64
	mapIDs                 []uint64
	fixedpointIDs          []uint64
	optimizerIDs           []uint64
	rcfNumberIDs           []uint64
	rcfOneID               uint64
	datatypeConstructorIDs []uint64
	datatypeListIDs        []uint64
}

// RegisterZ3GeneratedContextCommands installs the implemented context replay
// handlers. The replayer owns contexts left open by an incomplete log.
func RegisterZ3GeneratedContextCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{6, "Z3_mk_context", replayMkContext}, {7, "Z3_mk_context_rc", replayMkContextRC},
		{8, "Z3_del_context", replayDelContext}, {11, "Z3_update_param_value", replayUpdateContextParam}, {12, "Z3_interrupt", replayInterruptContext},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replayMkContext(r *Z3Replayer) error   { return replayCreateContext(r, false) }
func replayMkContextRC(r *Z3Replayer) error { return replayCreateContext(r, true) }
func replayCreateContext(r *Z3Replayer, userRef bool) error {
	id, err := r.Object(0)
	if err != nil {
		return err
	}
	var config *Config
	if id != 0 {
		config = r.configs[id]
		if config == nil {
			return ErrZ3ReplayReference
		}
	}
	return replayCreateContextWithConfig(r, config, userRef)
}

func replayCreateContextWithConfig(r *Z3Replayer, config *Config, userRef bool) error {
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	context, err := NewSMTContextWithConfig(config)
	if err != nil {
		return err
	}
	manager, err := context.Manager()
	if err != nil {
		return errors.Join(err, context.Close())
	}
	results, err := newAPIContextResults(manager, userRef)
	if err != nil {
		return errors.Join(err, context.Close())
	}
	if r.contexts == nil {
		r.contexts = make(map[uint64]*replayContextOwner)
	}
	r.nextObjectID++
	contextID := r.nextObjectID
	r.contexts[contextID] = &replayContextOwner{context: context, userRefCount: userRef, results: results}
	r.contextOrder = append(r.contextOrder, contextID)
	return r.StoreResult(contextID)
}
func replayContext(r *Z3Replayer, position uint32) (*replayContextOwner, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	context := r.contexts[id]
	if context == nil {
		return nil, ErrZ3ReplayReference
	}
	return context, nil
}
func replayDelContext(r *Z3Replayer) error {
	id, err := r.Object(0)
	if err != nil {
		return err
	}
	context := r.contexts[id]
	if context == nil {
		return ErrZ3ReplayReference
	}
	if err = context.close(r); err != nil {
		return err
	}
	delete(r.contexts, id)
	return nil
}
func replayUpdateContextParam(r *Z3Replayer) error {
	context, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	name, err := r.String(1)
	if err != nil {
		return err
	}
	value, err := r.String(2)
	if err != nil {
		return err
	}
	return context.context.UpdateParamValue(name, value)
}
func replayInterruptContext(r *Z3Replayer) error {
	context, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	return context.context.Interrupt()
}
