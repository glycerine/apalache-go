package rel

func (x *AAGExporter) lowerOutputs() error {
	predicates, e := x.rules.OutputPredicates()
	if e != nil {
		return e
	}
	outputs := make([]uint32, 0, len(predicates))
	for _, d := range predicates {
		terms, e := x.predicateTerms(d, x.current)
		if e != nil {
			return e
		}
		formula, e := x.andExpressions(terms)
		if e != nil {
			return e
		}
		id, e := x.builder.Expression(formula)
		if e != nil {
			return e
		}
		outputs = append(outputs, id)
	}
	id, e := x.builder.OrMany(outputs)
	if e != nil {
		return e
	}
	return x.builder.AddOutput(id)
}
