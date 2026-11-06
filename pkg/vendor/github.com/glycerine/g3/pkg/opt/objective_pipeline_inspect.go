package opt

import "github.com/glycerine/g3/pkg/ast"

type ObjectivePipelineInfo struct {
	SourceKind FormulaObjectiveKind
	Soft       bool
	Term       ast.ASTHandle
	SoftCount  int
	ID         string
	Converted  bool
	Negate     bool
}

func (p *ObjectivePipeline) ObjectiveCount() int {
	if p == nil {
		return 0
	}
	return len(p.objectives)
}
func (p *ObjectivePipeline) HardCount() int {
	if p == nil {
		return 0
	}
	return len(p.hard)
}
func (p *ObjectivePipeline) ObjectiveInfo(index int) (ObjectivePipelineInfo, error) {
	if err := p.open(); err != nil {
		return ObjectivePipelineInfo{}, err
	}
	if index < 0 || index >= len(p.objectives) {
		return ObjectivePipelineInfo{}, ast.ErrASTDomain
	}
	o := &p.objectives[index]
	result := ObjectivePipelineInfo{SourceKind: o.sourceKind, Soft: o.term == (ast.ASTRef{}), SoftCount: len(o.soft), ID: o.id, Converted: o.converted, Negate: o.negate}
	if o.term != (ast.ASTRef{}) {
		var err error
		result.Term, err = p.manager.RefGet(o.term)
		if err != nil {
			return ObjectivePipelineInfo{}, err
		}
	}
	return result, nil
}
