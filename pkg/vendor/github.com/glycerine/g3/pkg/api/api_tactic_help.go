package api

// Source: src/api/api_tactic.cpp:345-354.

import "errors"

// Help renders the tactic's collected parameter descriptions using the source
// parameter-description display, including documentation.
func (t *APITactic) Help() (text string, resultErr error) {
	descriptions, err := t.ParameterDescriptions()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
	return descriptions.value.Display(0, false, true)
}
