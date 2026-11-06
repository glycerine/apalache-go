package api

import "github.com/glycerine/g3/pkg/tactic"

// Source: src/api/api_tactic.cpp:286-322. Go returns copied slices in place
// of the native registry's indexed borrowed strings.

// ProbeDescriptors returns all retained named probes in native registration
// order. Both the slice and its values are caller-owned.
func (c *SMTContext) ProbeDescriptors() ([]APIProbeDescriptor, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return tactic.BuiltinProbeDescriptors(), nil
}

// ProbeNames is the name-only registry view used by frontends.
func (c *SMTContext) ProbeNames() ([]string, error) {
	descriptors, err := c.ProbeDescriptors()
	if err != nil {
		return nil, err
	}
	result := make([]string, len(descriptors))
	for i := range descriptors {
		result[i] = descriptors[i].Name
	}
	return result, nil
}
