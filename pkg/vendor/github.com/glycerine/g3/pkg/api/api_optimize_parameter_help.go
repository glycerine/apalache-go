package api

// Source: src/api/api_opt.cpp:261-271.

import "errors"

func (o *APIOptimize) ParameterHelp() (text string, resultErr error) {
	descriptions, err := o.ParameterDescriptions()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
	return descriptions.value.Display(0, false, true)
}
