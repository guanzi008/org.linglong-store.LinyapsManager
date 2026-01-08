package cmdwhitelist

import "fmt"

// ValidationError represents a command validation error.
type ValidationError struct {
	Command string
	Reason  string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("command %q validation failed: %s", e.Command, e.Reason)
}

// ValidateCommand validates a command and its arguments against the whitelist.
// Returns the actual program path to execute and validated args, or an error.
func ValidateCommand(cmdName string, args []string) (program string, validatedArgs []string, err error) {
	rule := GetRule(cmdName)
	if rule == nil {
		return "", nil, &ValidationError{
			Command: cmdName,
			Reason:  "command not in whitelist",
		}
	}

	// Delegate validation to the rule
	validatedArgs, err = rule.Validate(args)
	if err != nil {
		return "", nil, &ValidationError{
			Command: cmdName,
			Reason:  err.Error(),
		}
	}

	return rule.Program(), validatedArgs, nil
}

// NeedsSpecialEnv returns whether the command needs special environment setup.
func NeedsSpecialEnv(cmdName string) bool {
	rule := GetRule(cmdName)
	if rule == nil {
		return false
	}
	return rule.NeedsEnv()
}
