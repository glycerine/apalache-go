package api

// Source: generated src/api/api_commands.cpp, symbol creation commands.

// RegisterZ3GeneratedSymbolCommands installs integer and string symbol replay.
// The generated source ignores these functions' return values; later symbol
// arguments are serialized by value in the log.
func RegisterZ3GeneratedSymbolCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	if err := r.RegisterCommand(29, "Z3_mk_int_symbol", replayMkIntSymbol); err != nil {
		return err
	}
	if err := r.RegisterCommand(30, "Z3_mk_string_symbol", replayMkStringSymbol); err != nil {
		return err
	}
	for _, entry := range []struct {
		id   uint32
		name string
	}{{200, "Z3_get_symbol_kind"}, {201, "Z3_get_symbol_int"}, {202, "Z3_get_symbol_string"}} {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replaySymbolInspection(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayMkIntSymbol(r *Z3Replayer) error {
	context, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.Int(1)
	if err != nil {
		return err
	}
	_, err = context.context.SignedIntegerSymbol(value)
	return err
}
func replayMkStringSymbol(r *Z3Replayer) error {
	context, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.String(1)
	if err != nil {
		return err
	}
	_, err = context.context.StringSymbol(value)
	return err
}

func replaySymbolInspection(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := replayTypedSymbol(r, owner, 1)
	if err != nil {
		return err
	}
	switch id {
	case 200:
		_, err = owner.context.SymbolKind(value)
	case 201:
		_, err = owner.context.SymbolInteger(value)
	case 202:
		_, err = owner.context.SymbolString(value)
	}
	return err
}
