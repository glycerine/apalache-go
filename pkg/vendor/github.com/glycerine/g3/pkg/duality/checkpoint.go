package duality

import "github.com/glycerine/g3/pkg/util/z3exception"

// checkpoint cooperates with the shared manager resource/cancellation scope.
// The legacy frontend aborts the process on cancel; Go propagates the same
// cancellation message through owning query cleanup so callers can retry.
func (g *RPFP) checkpoint() error {
	if err := g.open(); err != nil {
		return err
	}
	canceled, err := g.context.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(g.context.manager.ResourceLimit().CancelMessage())
	}
	return nil
}
