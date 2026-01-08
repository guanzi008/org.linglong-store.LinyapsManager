package rules

import (
	"fmt"

	"linyapsmanager/internal/cmdwhitelist"
)

func init() {
	cmdwhitelist.Register(&killallRule{})
}

type killallRule struct{}

func (r *killallRule) Name() string {
	return "killall"
}

func (r *killallRule) Program() string {
	return "/usr/bin/killall"
}

func (r *killallRule) NeedsEnv() bool {
	return false
}

// allowedTargets lists the process names that can be killed.
var killallAllowedTargets = map[string]bool{
	"ll-cli": true,
}

// allowedSignals lists the signals that can be used.
var killallAllowedSignals = map[string]bool{
	"-15":      true, // SIGTERM
	"-SIGTERM": true,
	"-TERM":    true,
}

// blockedArgs lists arguments that are explicitly blocked.
var killallBlockedArgs = map[string]bool{
	"-u":     true,
	"--user": true,
}

func (r *killallRule) Validate(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("killall requires arguments")
	}

	// Check blocked args first
	for _, arg := range args {
		if killallBlockedArgs[arg] {
			return nil, fmt.Errorf("argument %q is not allowed", arg)
		}
	}

	// Parse args: can be "killall <target>" or "killall <signal> <target>"
	switch len(args) {
	case 1:
		// killall <target>
		target := args[0]
		if !killallAllowedTargets[target] {
			return nil, fmt.Errorf("process %q is not allowed to be killed", target)
		}
		return args, nil

	case 2:
		// killall <signal> <target>
		signal := args[0]
		target := args[1]

		if !killallAllowedSignals[signal] {
			return nil, fmt.Errorf("signal %q is not allowed", signal)
		}
		if !killallAllowedTargets[target] {
			return nil, fmt.Errorf("process %q is not allowed to be killed", target)
		}
		return args, nil

	default:
		return nil, fmt.Errorf("too many arguments: max 2, got %d", len(args))
	}
}
