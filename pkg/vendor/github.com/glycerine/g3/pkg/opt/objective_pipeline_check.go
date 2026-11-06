package opt

func (p *ObjectivePipeline) Check() (*ObjectivePipelineResult, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return p.checkArithmetic()
}
