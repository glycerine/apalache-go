package duality

import "io"

// SetProfileOutput borrows a destination for duality.profile reports. A nil
// writer restores the source stdout destination. Query results are separate.
func (b *Backend) SetProfileOutput(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	b.queries.profileOutput = out
	return nil
}

// SetReportOutput borrows the destination for the source report option.
func (b *Backend) SetReportOutput(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	b.queries.config.ReportOutput = out
	return nil
}
