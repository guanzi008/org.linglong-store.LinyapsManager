// Package cmdwhitelist provides command validation with a pluggable rule system.
package cmdwhitelist

import (
	"fmt"
	"sync"
)

// Rule defines the interface that each command rule must implement.
type Rule interface {
	// Name returns the command name (e.g., "ll-cli", "killall").
	Name() string

	// Program returns the actual executable path.
	Program() string

	// NeedsEnv returns whether this command needs special environment setup.
	NeedsEnv() bool

	// Validate validates the arguments and returns the validated args.
	// Returns an error if validation fails.
	Validate(args []string) (validatedArgs []string, err error)
}

// registry holds all registered rules.
var (
	registryMu sync.RWMutex
	rules      = make(map[string]Rule)
)

// Register adds a rule to the registry.
// This is typically called from init() in each rule file.
// Panics if a rule with the same name is already registered.
func Register(rule Rule) {
	registryMu.Lock()
	defer registryMu.Unlock()

	name := rule.Name()
	if _, exists := rules[name]; exists {
		panic(fmt.Sprintf("cmdwhitelist: rule %q already registered", name))
	}
	rules[name] = rule
}

// GetRule returns the rule for a command name.
// Returns nil if the command is not registered.
func GetRule(cmdName string) Rule {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return rules[cmdName]
}

// IsAllowed checks if a command name is registered.
func IsAllowed(cmdName string) bool {
	return GetRule(cmdName) != nil
}

// ListCommands returns all registered command names.
func ListCommands() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(rules))
	for name := range rules {
		names = append(names, name)
	}
	return names
}
