package spacer

// Source: src/muz/spacer/spacer_context.h and spacer_context.cpp pob_lt and
// pob_queue.

import "github.com/glycerine/g3/pkg/ast"

type ProofObligationQueue struct {
	root         *ProofObligation
	maximum      uint32
	minimumDepth uint32
	expanded     uint32
	restarts     uint64
	items        []*ProofObligation
}

func proofObligationConjunctCount(p *ProofObligation) (uint32, error) {
	post, err := p.Post()
	if err != nil {
		return 0, err
	}
	kind, err := p.transformer.manager.GetKind(post)
	if err != nil || kind != ast.ASTApp {
		return 1, err
	}
	family, err := p.transformer.manager.GetFamilyID(post)
	if err != nil || family != p.transformer.manager.BasicFamilyID() {
		return 1, err
	}
	op, err := p.transformer.manager.GetDeclKind(post)
	if err != nil || op != ast.ASTOpAnd {
		return 1, err
	}
	return p.transformer.manager.GetNumArgs(post)
}

func proofObligationLess(left, right *ProofObligation) (bool, error) {
	leftLevel, err := left.Level()
	if err != nil {
		return false, err
	}
	rightLevel, err := right.Level()
	if err != nil {
		return false, err
	}
	if leftLevel != rightLevel {
		return leftLevel < rightLevel, nil
	}
	leftDepth, err := left.Depth()
	if err != nil {
		return false, err
	}
	rightDepth, err := right.Depth()
	if err != nil {
		return false, err
	}
	if leftDepth != rightDepth {
		return leftDepth < rightDepth, nil
	}
	leftSize, err := proofObligationConjunctCount(left)
	if err != nil {
		return false, err
	}
	rightSize, err := proofObligationConjunctCount(right)
	if err != nil {
		return false, err
	}
	if leftSize != rightSize {
		return leftSize < rightSize, nil
	}
	leftPost, err := left.Post()
	if err != nil {
		return false, err
	}
	rightPost, err := right.Post()
	if err != nil {
		return false, err
	}
	leftID, err := left.transformer.manager.GetASTID(leftPost)
	if err != nil {
		return false, err
	}
	rightID, err := right.transformer.manager.GetASTID(rightPost)
	if err != nil {
		return false, err
	}
	if leftID != rightID {
		return leftID < rightID, nil
	}
	leftHead, leftHeadErr := left.transformer.Head()
	rightHead, rightHeadErr := right.transformer.Head()
	if leftHeadErr == nil && rightHeadErr == nil {
		leftHeadID, err := left.transformer.manager.GetASTID(leftHead)
		if err != nil {
			return false, err
		}
		rightHeadID, err := right.transformer.manager.GetASTID(rightHead)
		if err != nil {
			return false, err
		}
		if leftHeadID != rightHeadID {
			return leftHeadID < rightHeadID, nil
		}
	}
	return left.sequence < right.sequence, nil
}

func (q *ProofObligationQueue) Push(obligation *ProofObligation) error {
	if obligation == nil || obligation.valid() != nil {
		return ErrManagerOwner
	}
	index := len(q.items)
	q.items = append(q.items, obligation)
	for index > 0 {
		less, err := proofObligationLess(q.items[index], q.items[index-1])
		if err != nil {
			q.items = q.items[:len(q.items)-1]
			return err
		}
		if !less {
			break
		}
		q.items[index-1], q.items[index] = q.items[index], q.items[index-1]
		index--
	}
	return nil
}

func (q *ProofObligationQueue) Top() (*ProofObligation, error) {
	if len(q.items) == 0 {
		return nil, nil
	}
	top := q.items[0]
	level, err := top.Level()
	if err != nil {
		return nil, err
	}
	if level > q.maximum {
		return nil, nil
	}
	depth, err := top.Depth()
	if err != nil {
		return nil, err
	}
	if level == q.maximum && depth > q.minimumDepth {
		return nil, nil
	}
	return top, nil
}

func (q *ProofObligationQueue) Pop() (*ProofObligation, error) {
	if len(q.items) == 0 {
		return nil, ast.ErrASTDomain
	}
	result := q.items[0]
	copy(q.items, q.items[1:])
	q.items = q.items[:len(q.items)-1]
	return result, nil
}

func (q *ProofObligationQueue) Reset() error {
	q.items = q.items[:0]
	if q.root != nil {
		return q.Push(q.root)
	}
	return nil
}

func (q *ProofObligationQueue) SetRoot(root *ProofObligation) error {
	if root == nil || root.valid() != nil {
		return ErrManagerOwner
	}
	q.root = root
	q.maximum = root.level
	q.minimumDepth = root.depth
	return q.Reset()
}

func (q *ProofObligationQueue) Root() (*ProofObligation, error) {
	if q.root == nil || q.root.valid() != nil {
		return nil, ErrManagerOwner
	}
	return q.root, nil
}

func (q *ProofObligationQueue) IsRoot(obligation *ProofObligation) bool {
	return q != nil && q.root != nil && q.root == obligation
}

func (q *ProofObligationQueue) IncrementLevel() error {
	if q.root == nil {
		return ErrManagerOwner
	}
	q.maximum++
	q.minimumDepth++
	if len(q.items) == 0 {
		return q.Push(q.root)
	}
	return nil
}

func (q *ProofObligationQueue) MaximumLevel() uint32  { return q.maximum }
func (q *ProofObligationQueue) MinimumDepth() uint32  { return q.minimumDepth }
func (q *ProofObligationQueue) ExpandedLevel() uint32 { return q.expanded }
func (q *ProofObligationQueue) Len() int              { return len(q.items) }

// RestartCount returns the number of scheduled queue restarts.
func (q *ProofObligationQueue) RestartCount() uint64 { return q.restarts }
