package rel

func (x *AAGExporter) ensureArgumentLatches(n int) error {
	for len(x.argCurrent) < n {
		current, err := x.fresh("latch_var")
		if err != nil {
			return err
		}
		next, err := x.fresh("latch_varp")
		if err != nil {
			return err
		}
		if _, err = x.builder.internal(current); err != nil {
			return err
		}
		if _, err = x.builder.Input(next); err != nil {
			return err
		}
		x.argCurrent = append(x.argCurrent, current)
		x.argNext = append(x.argNext, next)
	}
	return nil
}
