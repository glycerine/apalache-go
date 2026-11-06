package api

// Source: generated api_commands.cpp and src/api/api_qe.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func registerZ3GeneratedQECommands(r *Z3Replayer) error {
	names := []string{"Z3_qe_model_project", "Z3_qe_model_project_skolem", "Z3_model_extrapolate", "Z3_qe_lite"}
	for i, name := range names {
		id := uint32(657 + i)
		if err := r.RegisterCommand(id, name, func(r *Z3Replayer) error { return replayQECommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replayQECommand(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	var result ast.ASTRef
	if id == 660 {
		vector, e := replayVectorAt(r, owner, 1)
		if e != nil {
			return e
		}
		body, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		variables, e := vector.value.Values()
		if e != nil {
			return e
		}
		var remaining []ast.ASTHandle
		result, remaining, err = owner.context.QELite(variables, body.handle)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, owner.results.manager.DelRef(&result)) }()
		if len(remaining) < len(variables) {
			pins, e := apiQEOwnRefs(owner.results.manager, remaining)
			if e != nil {
				return e
			}
			defer func() { err = errors.Join(err, apiQECloseRefs(owner.results.manager, &pins)) }()
			if e = vector.value.Resize(0); e != nil {
				return e
			}
			for _, value := range remaining {
				if e = vector.value.Push(value); e != nil {
					return e
				}
			}
		}
	} else {
		m, e := replayModelAt(r, owner, 1)
		if e != nil {
			return e
		}
		if id == 659 {
			body, e := r.astObject(owner, 2)
			if e != nil {
				return e
			}
			result, err = owner.context.ModelExtrapolate(m.value.core, body.handle)
		} else {
			bounds, e := replayASTArray(r, owner, 2, 3)
			if e != nil {
				return e
			}
			body, e := r.astObject(owner, 4)
			if e != nil {
				return e
			}
			if id == 657 {
				result, err = owner.context.QEModelProject(m.value.core, bounds, body.handle)
			} else {
				mapID, e := r.Object(5)
				if e != nil {
					return e
				}
				mapping := r.maps[mapID]
				if mapping == nil || mapping.owner != owner || mapping.object.open() != nil {
					return ErrZ3ReplayReference
				}
				result, err = owner.context.QEModelProjectSkolem(m.value.core, bounds, body.handle, mapping.value)
			}
		}
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, owner.results.manager.DelRef(&result)) }()
	}
	value, err := owner.results.manager.RefGet(result)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}
