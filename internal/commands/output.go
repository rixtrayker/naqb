package commands

import "os"

// OutputFlags holds the global output verbosity settings.
var Output OutputFlags

// OutputFlags controls verbosity and color across all commands.
type OutputFlags struct {
	Verbose bool
	Quiet   bool
	NoColor bool
}

// IsVerbose returns true if verbose output is enabled.
func (f OutputFlags) IsVerbose() bool { return f.Verbose }

// IsQuiet returns true if quiet mode is enabled (suppress non-essential output).
func (f OutputFlags) IsQuiet() bool { return f.Quiet }

// ColorDisabled returns true if color output should be suppressed.
// Respects --no-color flag and the NO_COLOR environment variable.
func (f OutputFlags) ColorDisabled() bool {
	if f.NoColor {
		return true
	}
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}
