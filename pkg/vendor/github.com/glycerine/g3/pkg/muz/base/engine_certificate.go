package base

// Source: src/muz/base/dl_engine_base.h engine_base::display_certificate.

import "io"

func (e *EngineBase) DisplayCertificate(out io.Writer) error {
	if x := e.open(); x != nil {
		return x
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if capability, ok := e.core.(EngineCertificate); ok {
		return capability.DisplayCertificate(out)
	}
	return e.unsupported("certificates")
}
