package rules

import (
	"fmt"

	"linyapsmanager/internal/cmdwhitelist"
)

func init() {
	cmdwhitelist.Register(&pkexecRule{})
}

type pkexecRule struct{}

func (r *pkexecRule) Name() string {
	return "pkexec"
}

func (r *pkexecRule) Program() string {
	return "/usr/bin/pkexec"
}

func (r *pkexecRule) NeedsEnv() bool {
	return false
}

const pkexecMaxArgs = 30

func (r *pkexecRule) Validate(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("pkexec requires a command to execute")
	}

	if len(args) > pkexecMaxArgs {
		return nil, fmt.Errorf("too many arguments: max %d, got %d", pkexecMaxArgs, len(args))
	}

	// The first argument is the nested command to execute
	nestedCmd := args[0]
	nestedArgs := args[1:]

	// Recursively validate the nested command
	nestedRule := cmdwhitelist.GetRule(nestedCmd)
	if nestedRule == nil {
		return nil, fmt.Errorf("pkexec target command %q is not allowed", nestedCmd)
	}

	// Validate nested command's arguments
	validatedNestedArgs, err := nestedRule.Validate(nestedArgs)
	if err != nil {
		return nil, fmt.Errorf("pkexec target command %q invalid: %v", nestedCmd, err)
	}

	// Replace command name with actual program path
	result := make([]string, len(validatedNestedArgs)+1)
	result[0] = nestedRule.Program()
	copy(result[1:], validatedNestedArgs)

	return result, nil
}
