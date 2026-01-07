package cmdwhitelist

import (
	"fmt"
	"strings"
)

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
	cfg := GetConfig(cmdName)
	if cfg == nil {
		return "", nil, &ValidationError{
			Command: cmdName,
			Reason:  "command not in whitelist",
		}
	}

	// Check required args
	if cfg.RequireArgs && len(args) == 0 {
		return "", nil, &ValidationError{
			Command: cmdName,
			Reason:  "command requires arguments",
		}
	}

	// Check max args
	if cfg.MaxArgs > 0 && len(args) > cfg.MaxArgs {
		return "", nil, &ValidationError{
			Command: cmdName,
			Reason:  fmt.Sprintf("too many arguments: max %d, got %d", cfg.MaxArgs, len(args)),
		}
	}

	// Check blocked args
	for _, arg := range args {
		for _, blocked := range cfg.BlockedArgs {
			if arg == blocked || strings.HasPrefix(arg, blocked+"=") {
				return "", nil, &ValidationError{
					Command: cmdName,
					Reason:  fmt.Sprintf("argument %q is not allowed", arg),
				}
			}
		}
	}

	// Check subcommands if configured
	if len(cfg.AllowedSubcmds) > 0 && len(args) > 0 {
		subcmd := args[0]
		// Skip flags when looking for subcommand
		if !strings.HasPrefix(subcmd, "-") {
			allowed := false
			for _, s := range cfg.AllowedSubcmds {
				if s == subcmd {
					allowed = true
					break
				}
			}
			if !allowed {
				return "", nil, &ValidationError{
					Command: cmdName,
					Reason:  fmt.Sprintf("subcommand %q is not allowed", subcmd),
				}
			}
		} else {
			// First arg is a flag, check if it's an allowed subcommand-like flag (e.g., --version)
			allowed := false
			for _, s := range cfg.AllowedSubcmds {
				if s == subcmd {
					allowed = true
					break
				}
			}
			if !allowed {
				return "", nil, &ValidationError{
					Command: cmdName,
					Reason:  fmt.Sprintf("flag %q is not allowed as first argument", subcmd),
				}
			}
		}
	}

	// Special handling for pkexec: validate the nested command
	if cmdName == "pkexec" && len(args) > 0 {
		nestedCmd := args[0]
		nestedArgs := args[1:]

		// Validate nested command
		nestedProgram, _, err := ValidateCommand(nestedCmd, nestedArgs)
		if err != nil {
			return "", nil, &ValidationError{
				Command: cmdName,
				Reason:  fmt.Sprintf("pkexec target command invalid: %v", err),
			}
		}

		// Replace the command name with actual program path in args
		validatedArgs = make([]string, len(args))
		validatedArgs[0] = nestedProgram
		copy(validatedArgs[1:], nestedArgs)
		return cfg.Program, validatedArgs, nil
	}

	return cfg.Program, args, nil
}

// NeedsSpecialEnv returns whether the command needs special environment setup.
func NeedsSpecialEnv(cmdName string) bool {
	cfg := GetConfig(cmdName)
	if cfg == nil {
		return false
	}
	return cfg.NeedsEnv
}
