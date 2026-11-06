package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// retainTrail models ddnf::imp::m_trail: references survive query-local cache
// cleanup and remain owned by the DDNF engine until it is replaced or closed.
func (s *ddnfEngineState) retainTrail(references []ast.ASTRef) {
	s.trail = append(s.trail, references...)
}

func (s *ddnfEngineState) transferTrail(destination *ddnfEngineState) {
	if s == nil || destination == nil || len(s.trail) == 0 {
		return
	}
	destination.retainTrail(s.trail)
	s.trail = nil
}

// closeTrail keeps references whose release did not consume ownership so a
// later close attempt can retry them. Successfully consumed entries are
// omitted even when their release reports a secondary cleanup error.
func (s *ddnfEngineState) closeTrail(manager *ast.ASTManager) (result error) {
	if s == nil || len(s.trail) == 0 {
		return nil
	}
	remaining := make([]ast.ASTRef, 0, len(s.trail))
	for index := len(s.trail) - 1; index >= 0; index-- {
		reference := s.trail[index]
		err := manager.DelRef(&reference)
		result = errors.Join(result, err)
		if reference != (ast.ASTRef{}) {
			remaining = append(remaining, reference)
		}
	}
	for left, right := 0, len(remaining)-1; left < right; left, right = left+1, right-1 {
		remaining[left], remaining[right] = remaining[right], remaining[left]
	}
	s.trail = remaining
	return result
}
