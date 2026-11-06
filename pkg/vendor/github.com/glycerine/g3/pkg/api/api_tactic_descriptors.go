package api

// Source: src/api/api_tactic.cpp tactic enumeration and description APIs.

// APITacticDescriptor is copied public tactic registration metadata.
type APITacticDescriptor struct {
	Name        string
	Description string
}

// TacticDescriptors returns all retained tactic metadata in registry order.
func (c *SMTContext) TacticDescriptors() ([]APITacticDescriptor, error) {
	names, err := c.TacticNames()
	if err != nil {
		return nil, err
	}
	result := make([]APITacticDescriptor, len(names))
	for i, name := range names {
		description, descriptionErr := c.TacticDescription(name)
		if descriptionErr != nil {
			return nil, descriptionErr
		}
		result[i] = APITacticDescriptor{Name: name, Description: description}
	}
	return result, nil
}
