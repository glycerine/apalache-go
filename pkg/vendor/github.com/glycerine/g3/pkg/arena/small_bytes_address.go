package arena

// AddressText supplies the actual live payload address for source diagnostics.
// It does not expose borrowed storage or keep the allocation alive.

import "fmt"

func (a *SmallBytes) AddressText(r SmallBytesRef) (string, error) {
	d, err := a.descriptor(r)
	if err != nil {
		return "", err
	}
	if r == (SmallBytesRef{}) {
		return "0", nil
	}
	if d.length == 0 {
		return "0", nil
	}
	return fmt.Sprintf("%p", &a.chunks[d.page].data[d.offset]), nil
}
