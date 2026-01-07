// Package cmdwhitelist defines allowed commands and validation rules.
package cmdwhitelist

// CommandConfig defines the configuration for an allowed command.
type CommandConfig struct {
	// Program is the actual executable path or name.
	Program string

	// AllowedSubcmds lists allowed subcommands. Empty means no restriction on subcommands.
	AllowedSubcmds []string

	// BlockedArgs lists arguments that are explicitly blocked.
	BlockedArgs []string

	// RequireArgs indicates whether the command requires at least one argument.
	RequireArgs bool

	// MaxArgs limits the maximum number of arguments. 0 means no limit.
	MaxArgs int

	// NeedsEnv indicates whether this command needs special environment setup (like ll-cli).
	NeedsEnv bool
}

// AllowedCommands maps command names (as invoked by the client) to their configurations.
// The key is the name the client uses (e.g., "ll-cli", "killall").
var AllowedCommands = map[string]CommandConfig{
	"ll-cli": {
		Program: "ll-cli",
		AllowedSubcmds: []string{
			"--version", "--help",
			"version", "help",
			"repo",
			"list",
			"search",
			"info",
			"ps",
			"install",
			"uninstall",
			"run",
			"kill",
			"prune",
			"exec",
			"content",
		},
		BlockedArgs: []string{
			// Block potentially dangerous operations
		},
		NeedsEnv: true,
		MaxArgs:  20,
	},

	"killall": {
		Program:     "/usr/bin/killall",
		RequireArgs: true,
		MaxArgs:     10,
		BlockedArgs: []string{
			"-u", "--user", // Don't allow killing by user
		},
	},

	"kill": {
		Program:     "/usr/bin/kill",
		RequireArgs: true,
		MaxArgs:     10,
	},

	"pkexec": {
		Program:     "/usr/bin/pkexec",
		RequireArgs: true,
		MaxArgs:     30,
		// Note: pkexec args will be validated recursively
	},
}

// GetConfig returns the configuration for a command name.
// Returns nil if the command is not allowed.
func GetConfig(cmdName string) *CommandConfig {
	if cfg, ok := AllowedCommands[cmdName]; ok {
		return &cfg
	}
	return nil
}

// IsAllowed checks if a command name is in the whitelist.
func IsAllowed(cmdName string) bool {
	_, ok := AllowedCommands[cmdName]
	return ok
}

// GetProgram returns the actual executable path for a command name.
// Returns empty string if not allowed.
func GetProgram(cmdName string) string {
	if cfg, ok := AllowedCommands[cmdName]; ok {
		return cfg.Program
	}
	return ""
}
