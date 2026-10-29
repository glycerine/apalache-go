package pattern

// Source: src/ast/pattern/pattern_inference.cpp:270-320.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type candidateInfo struct {
	freeVars map[uint32]struct{}
	size     uint32
}

// patternCandidates is the retained candidate vector and metadata subset of
// pattern_inference_cfg. It owns each candidate AST reference; exclusions and
// returned filter slices borrow handles from the surrounding quantifier.
type patternCandidates struct {
	manager            *ast.ASTManager
	matcher            *smallerPattern
	database           *exprPatternMatcher
	candidates         []ast.ASTHandle
	info               map[ast.ASTHandle]candidateInfo
	preferred          map[ast.ASTHandle]struct{}
	visited            map[ast.ASTHandle]struct{}
	todo               []ast.ASTHandle
	noPatterns         []ast.ASTHandle
	collectRefs        []collectedRef
	pendingPatternRefs []ast.ASTRef
	numBindings        uint32
	blockLoops         bool
	avoidSkolems       bool
	nestedArithOnly    bool
	forbiddenFamilies  map[ast.FamilyID]struct{}
	traceTags          *diagnostic.TraceTags
	closed             bool
}

func newPatternCandidates(m *ast.ASTManager, numBindings uint32, noPatterns []ast.ASTHandle, blockLoops bool) (*patternCandidates, error) {
	matcher, err := newSmallerPattern(m)
	if err != nil {
		return nil, err
	}
	database, err := newExprPatternMatcher(m)
	if err != nil {
		return nil, err
	}
	return &patternCandidates{
		manager: m, matcher: matcher, database: database, info: make(map[ast.ASTHandle]candidateInfo), preferred: make(map[ast.ASTHandle]struct{}), visited: make(map[ast.ASTHandle]struct{}),
		noPatterns: noPatterns, numBindings: numBindings, blockLoops: blockLoops,
		avoidSkolems: true, nestedArithOnly: true, forbiddenFamilies: make(map[ast.FamilyID]struct{}),
	}, nil
}

func (c *patternCandidates) addCandidate(n ast.ASTHandle, freeVars []uint32, size uint32) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	for _, excluded := range c.noPatterns {
		if n == excluded {
			return nil
		}
	}
	if _, found := c.info[n]; found {
		return nil
	}
	kind, err := c.manager.GetKind(n)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	if err := c.manager.IncRef(n); err != nil {
		return err
	}
	vars := make(map[uint32]struct{}, len(freeVars))
	for _, v := range freeVars {
		vars[v] = struct{}{}
	}
	c.info[n] = candidateInfo{freeVars: vars, size: size}
	c.candidates = append(c.candidates, n)
	return nil
}

func sameFreeVars(a, b map[uint32]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for v := range a {
		if _, found := b[v]; !found {
			return false
		}
	}
	return true
}

// filterLooping preserves the source's outer insertion order. A candidate is
// erased from the metadata map only after a same-variable candidate is a
// strict instance of it; the owned candidate vector remains intact.
func (c *patternCandidates) filterLooping() ([]ast.ASTHandle, error) {
	if c == nil || c.closed {
		return nil, ast.ErrASTHandle
	}
	c.matcher.traceTags = c.traceTags
	result := make([]ast.ASTHandle, 0, len(c.candidates))
	for i, first := range c.candidates {
		firstInfo, found := c.info[first]
		if !found {
			return nil, ast.ErrASTDomain
		}
		if !c.blockLoops {
			result = append(result, first)
			continue
		}
		remove := false
		for j, second := range c.candidates {
			if i == j {
				continue
			}
			secondInfo, found := c.info[second]
			if !found || !sameFreeVars(firstInfo.freeVars, secondInfo.freeVars) {
				continue
			}
			forward, err := c.matcher.smaller(c.numBindings, first, second)
			if err != nil {
				return nil, err
			}
			if !forward {
				continue
			}
			reverse, err := c.matcher.smaller(c.numBindings, second, first)
			if err != nil {
				return nil, err
			}
			if !reverse {
				remove = true
				break
			}
		}
		if remove {
			delete(c.info, first)
		} else {
			result = append(result, first)
		}
	}
	return result, nil
}

// resetCandidates follows mk_patterns' end-of-call metadata reset and the
// owning app_ref_vector's forward release order. A stopped release leaves its
// unconsumed suffix for the next reset or close attempt.
func (c *patternCandidates) resetCandidates() error {
	if err := c.releaseCollectRefs(); err != nil {
		return err
	}
	for len(c.candidates) != 0 {
		h := c.candidates[0]
		consumed, err := c.manager.DecRefConsumed(h)
		if consumed {
			c.candidates[0] = ast.ASTHandle{}
			c.candidates = c.candidates[1:]
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	clear(c.info)
	clear(c.visited)
	c.todo = nil
	c.noPatterns = nil
	return nil
}

func (c *patternCandidates) close() error {
	if c == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if err := c.releasePendingPatternRefs(); err != nil {
		return err
	}
	if err := c.resetCandidates(); err != nil {
		return err
	}
	if c.database != nil {
		if err := c.database.Close(); err != nil {
			return err
		}
		c.database = nil
	}
	clear(c.preferred)
	c.closed = true
	return nil
}
