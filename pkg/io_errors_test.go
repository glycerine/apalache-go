package apalache

import (
	"errors"
	"testing"
)

func TestIOErrorsPreserveTypedClassificationAndMessages(t *testing.T) {
	configuration := &ConfigurationError{Message: "bad option"}
	pretty := &PrettyPrinterError{Message: "cannot render"}
	if configuration.Error() != "bad option" || pretty.Error() != "cannot render" {
		t.Fatalf("error messages = %q, %q", configuration, pretty)
	}
	var configurationTarget *ConfigurationError
	var prettyTarget *PrettyPrinterError
	if !errors.As(configuration, &configurationTarget) || errors.As(configuration, &prettyTarget) ||
		!errors.As(pretty, &prettyTarget) || errors.As(pretty, &configurationTarget) {
		t.Fatal("I/O error types were not independently classifiable")
	}
}
