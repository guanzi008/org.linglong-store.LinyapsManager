// Package rules contains individual command validation rules.
//
// Each file in this package defines a single command rule that implements
// the cmdwhitelist.Rule interface. Rules are automatically registered
// via their init() functions.
//
// To add a new command:
//  1. Create a new file named after the command (e.g., mycommand.go)
//  2. Define a struct implementing the Rule interface
//  3. Register it in init() with cmdwhitelist.Register(&myRule{})
//
// Example:
//
//	package rules
//
//	import "linyapsmanager/internal/cmdwhitelist"
//
//	func init() {
//	    cmdwhitelist.Register(&myRule{})
//	}
//
//	type myRule struct{}
//
//	func (r *myRule) Name() string    { return "mycommand" }
//	func (r *myRule) Program() string { return "/usr/bin/mycommand" }
//	func (r *myRule) NeedsEnv() bool  { return false }
//	func (r *myRule) Validate(args []string) ([]string, error) {
//	    // Your validation logic here
//	    return args, nil
//	}
package rules
