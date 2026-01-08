package rules

import (
	"fmt"
	"strings"

	"linyapsmanager/internal/cmdwhitelist"
)

func init() {
	cmdwhitelist.Register(&llcliRule{})
}

type llcliRule struct{}

func (r *llcliRule) Name() string {
	return "ll-cli"
}

func (r *llcliRule) Program() string {
	return "ll-cli"
}

func (r *llcliRule) NeedsEnv() bool {
	return true
}

// allowedSubcmds lists all allowed subcommands for ll-cli.
var llcliAllowedSubcmds = map[string]bool{
	"--version": true,
	"--help":    true,
	"version":   true,
	"help":      true,
	"repo":      true,
	"list":      true,
	"search":    true,
	"info":      true,
	"ps":        true,
	"install":   true,
	"uninstall": true,
	"run":       true,
	"kill":      true,
	"prune":     true,
	"exec":      true,
	"content":   true,
}

const llcliMaxArgs = 20

func (r *llcliRule) Validate(args []string) ([]string, error) {
	// Check max args
	if len(args) > llcliMaxArgs {
		return nil, fmt.Errorf("too many arguments: max %d, got %d", llcliMaxArgs, len(args))
	}

	// Check subcommand if present
	if len(args) > 0 {
		subcmd := args[0]
		// Find the first non-flag argument as subcommand
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				subcmd = arg
				break
			}
		}

		// If first arg is a flag, check it's allowed
		if strings.HasPrefix(args[0], "-") {
			if !llcliAllowedSubcmds[args[0]] {
				return nil, fmt.Errorf("flag %q is not allowed as first argument", args[0])
			}
		} else {
			// First arg is a subcommand
			if !llcliAllowedSubcmds[subcmd] {
				return nil, fmt.Errorf("subcommand %q is not allowed", subcmd)
			}
		}
	}

	return args, nil
}
