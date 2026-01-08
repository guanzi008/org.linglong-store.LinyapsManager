// Package cmdwhitelist defines allowed commands and validation rules.
//
// This package uses a pluggable rule system. Each command has its own
// rule file in the rules/ subdirectory. See rules/doc.go for details
// on how to add new commands.
package cmdwhitelist

// GetProgram returns the actual executable path for a command name.
// Returns empty string if not allowed.
func GetProgram(cmdName string) string {
	rule := GetRule(cmdName)
	if rule == nil {
		return ""
	}
	return rule.Program()
}
