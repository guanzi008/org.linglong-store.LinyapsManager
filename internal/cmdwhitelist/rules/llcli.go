package rules

import (
	"fmt"
	"log"
	"os/exec"
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

// commonFlags lists common global flags that can appear anywhere in the command line.
var llcliCommonFlags = map[string]bool{
	"--json":    true,
	"--verbose": true,
	"--debug":   true,
	"--no-dbus": true,
}

const llcliMaxArgs = 20

func (r *llcliRule) Validate(args []string) ([]string, error) {
	// Check max args
	if len(args) > llcliMaxArgs {
		return nil, fmt.Errorf("too many arguments: max %d, got %d", llcliMaxArgs, len(args))
	}

	// Check subcommand if present
	if len(args) > 0 {
		// Find the first argument that's not a common flag
		var subcmd string
		for _, arg := range args {
			// Skip common global flags
			if llcliCommonFlags[arg] {
				continue
			}
			// Skip option-value pairs (e.g., --repo <url>)
			if strings.HasPrefix(arg, "-") && !llcliAllowedSubcmds[arg] {
				// Unknown flag - skip it (ll-cli will validate)
				continue
			}
			// Found a subcommand or special flag
			subcmd = arg
			break
		}

		// If we found a subcommand or special flag, validate it
		if subcmd != "" && !llcliAllowedSubcmds[subcmd] {
			return nil, fmt.Errorf("subcommand %q is not allowed", subcmd)
		}

		// Special handling: kill the app before installing com.dongpl.linglong-store.v2
		if subcmd == "install" && len(args) >= 2 && args[1] == "com.dongpl.linglong-store.v2" {
			log.Printf("[INFO] Pre-killing com.dongpl.linglong-store.v2 before install")

			// Execute: ll-cli kill -s 9 com.dongpl.linglong-store.v2
			killCmd := exec.Command("ll-cli", "kill", "-s", "9", "com.dongpl.linglong-store.v2")
			if output, err := killCmd.CombinedOutput(); err != nil {
				log.Printf("[WARN] kill failed (app may not be running): %v, output: %s", err, string(output))
			} else {
				log.Printf("[INFO] kill succeeded: %s", string(output))
			}
		}
	}

	return args, nil
}
