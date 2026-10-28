package apalache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApalacheConfigLoader selects at most one JSON configuration file and merges
// it below a primary configuration. Defaults are deliberately applied later.
type ApalacheConfigLoader struct {
	WorkingDirectory  string
	UserHomeDirectory string
}

func NewApalacheConfigLoader(workingDirectory, userHomeDirectory string) ApalacheConfigLoader {
	if absolute, err := filepath.Abs(workingDirectory); err == nil {
		workingDirectory = absolute
	}
	if absolute, err := filepath.Abs(userHomeDirectory); err == nil {
		userHomeDirectory = absolute
	}
	return ApalacheConfigLoader{
		WorkingDirectory:  filepath.Clean(workingDirectory),
		UserHomeDirectory: filepath.Clean(userHomeDirectory),
	}
}

// Load selects an explicit file, a working-directory-local file, or the
// user-wide file, in that order. No lower-precedence candidate is inspected
// after a match.
func (loader ApalacheConfigLoader) Load(primary ApalacheConfig) ConfigParseResult[ApalacheConfig] {
	selected, label, found, selectionError := loader.selectedFile(primary)
	if selectionError != "" {
		return ConfigFailureMessage[ApalacheConfig](selectionError)
	}
	if !found {
		return ConfigSuccess(primary)
	}
	decoded := decodeApalacheConfigFile(selected, label)
	if !decoded.IsSuccess() {
		return ConfigFailureFrom[ApalacheConfig](decoded)
	}
	return ConfigSuccess(primary.MergeWithLower(decoded.RequireValue()), decoded.Warnings()...)
}

func (loader ApalacheConfigLoader) selectedFile(primary ApalacheConfig) (path, label string, found bool, selectionError string) {
	if primary.Context.ConfigFile != nil {
		path = *primary.Context.ConfigFile
		if strings.HasSuffix(filepath.Base(path), LegacyConfigExtension) {
			return "", "", false, legacyApalacheConfigFilenameError(path)
		}
		return path, path, true, ""
	}
	local := filepath.Join(loader.WorkingDirectory, LocalConfigFilename)
	if pathExists(local) {
		return local, LocalConfigFilename, true, ""
	}
	global := filepath.Join(loader.UserHomeDirectory, TLAPlusConfigDirectory, GlobalConfigFilename)
	if pathExists(global) {
		return global, global, true, ""
	}
	return "", "", false, ""
}

func decodeApalacheConfigFile(path, label string) ConfigParseResult[ApalacheConfig] {
	if !pathExists(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			absolute = path
		}
		return ConfigFailureMessage[ApalacheConfig]("Configuration file not found: " + absolute)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ConfigFailureMessage[ApalacheConfig](fmt.Sprintf("%s: Could not read configuration: %v", label, err))
	}
	decoded := ParseApalacheConfigJSON(string(data), label)
	if decoded.IsSuccess() {
		return decoded
	}
	errors := decoded.Errors()
	for index, message := range errors {
		if !strings.HasPrefix(message, label) {
			errors[index] = label + ": " + message
		}
	}
	return ConfigFailure[ApalacheConfig](errors, decoded.Warnings()...)
}

func legacyApalacheConfigFilenameError(path string) string {
	filename := filepath.Base(path)
	replacement := filepath.Join(filepath.Dir(path), strings.TrimSuffix(filename, LegacyConfigExtension)+JSONConfigExtension)
	if pathExists(replacement) {
		return fmt.Sprintf("Legacy Apalache configuration file %s is not supported. Remove it; use %s.", path, replacement)
	}
	return fmt.Sprintf("Legacy Apalache configuration file %s is not supported. Rename it to %s and convert it to strict JSON.", path, replacement)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// LoadApalacheConfig uses the process working directory and user home.
func LoadApalacheConfig(primary ApalacheConfig) ConfigParseResult[ApalacheConfig] {
	workingDirectory, err := os.Getwd()
	if err != nil {
		workingDirectory = "."
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		userHome = "."
	}
	return NewApalacheConfigLoader(workingDirectory, userHome).Load(primary)
}
