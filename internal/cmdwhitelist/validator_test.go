package cmdwhitelist_test

import (
	"testing"

	"linyapsmanager/internal/cmdwhitelist"
	_ "linyapsmanager/internal/cmdwhitelist/rules" // Register command rules
)

func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    bool
	}{
		{"ll-cli allowed", "ll-cli", true},
		{"killall allowed", "killall", true},
		{"kill allowed", "kill", true},
		{"pkexec allowed", "pkexec", true},
		{"random not allowed", "random-cmd", false},
		{"rm not allowed", "rm", false},
		{"sudo not allowed", "sudo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cmdwhitelist.IsAllowed(tt.cmdName); got != tt.want {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.cmdName, got, tt.want)
			}
		})
	}
}

func TestGetProgram(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    string
	}{
		{"ll-cli program", "ll-cli", "ll-cli"},
		{"killall program", "killall", "/usr/bin/killall"},
		{"kill program", "kill", "/usr/bin/kill"},
		{"pkexec program", "pkexec", "/usr/bin/pkexec"},
		{"unknown returns empty", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cmdwhitelist.GetProgram(tt.cmdName); got != tt.want {
				t.Errorf("GetProgram(%q) = %q, want %q", tt.cmdName, got, tt.want)
			}
		})
	}
}

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmdName     string
		args        []string
		wantProgram string
		wantErr     bool
	}{
		// Basic allowed commands
		{"ll-cli list", "ll-cli", []string{"list"}, "ll-cli", false},
		{"ll-cli install", "ll-cli", []string{"install", "com.example.app"}, "ll-cli", false},
		{"ll-cli version", "ll-cli", []string{"--version"}, "ll-cli", false},
		{"ll-cli search", "ll-cli", []string{"search", "firefox"}, "ll-cli", false},
		// Kill commands
		{"kill with pid", "kill", []string{"12345"}, "/usr/bin/kill", false},
		{"kill with signal", "kill", []string{"-9", "12345"}, "/usr/bin/kill", false},
		{"killall ll-cli", "killall", []string{"ll-cli"}, "/usr/bin/killall", false},
		{"killall with signal", "killall", []string{"-15", "ll-cli"}, "/usr/bin/killall", false},
		// pkexec with nested command
		{"pkexec ll-cli", "pkexec", []string{"ll-cli", "install", "app"}, "/usr/bin/pkexec", false},
		// Errors
		{"unknown command", "unknown", []string{}, "", true},
		{"ll-cli unknown subcmd", "ll-cli", []string{"unknown"}, "", true},
		{"killall requires args", "killall", []string{}, "", true},
		{"pkexec requires args", "pkexec", []string{}, "", true},
		{"pkexec with blocked cmd", "pkexec", []string{"rm", "-rf", "/"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, _, err := cmdwhitelist.ValidateCommand(tt.cmdName, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateCommand(%q, %v) expected error, got nil", tt.cmdName, tt.args)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateCommand(%q, %v) unexpected error: %v", tt.cmdName, tt.args, err)
				return
			}
			if program != tt.wantProgram {
				t.Errorf("ValidateCommand(%q, %v) program = %q, want %q", tt.cmdName, tt.args, program, tt.wantProgram)
			}
		})
	}
}

func TestValidateCommand_BlockedArgs(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		args    []string
		wantErr bool
	}{
		{"killall blocked -u", "killall", []string{"-u", "root"}, true},
		{"killall blocked --user", "killall", []string{"--user", "root"}, true},
		{"killall ll-cli ok", "killall", []string{"ll-cli"}, false},
		{"killall firefox blocked", "killall", []string{"firefox"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := cmdwhitelist.ValidateCommand(tt.cmdName, tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateCommand(%q, %v) expected error for blocked arg", tt.cmdName, tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateCommand(%q, %v) unexpected error: %v", tt.cmdName, tt.args, err)
			}
		})
	}
}

func TestValidateCommand_MaxArgs(t *testing.T) {
	// Create args exceeding max
	manyArgs := make([]string, 25)
	for i := range manyArgs {
		manyArgs[i] = "arg"
	}
	_, _, err := cmdwhitelist.ValidateCommand("ll-cli", append([]string{"list"}, manyArgs...))
	if err == nil {
		t.Error("ValidateCommand with too many args should return error")
	}
}

func TestNeedsSpecialEnv(t *testing.T) {
	tests := []struct {
		cmdName string
		want    bool
	}{
		{"ll-cli", true},
		{"killall", false},
		{"kill", false},
		{"pkexec", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmdName, func(t *testing.T) {
			if got := cmdwhitelist.NeedsSpecialEnv(tt.cmdName); got != tt.want {
				t.Errorf("NeedsSpecialEnv(%q) = %v, want %v", tt.cmdName, got, tt.want)
			}
		})
	}
}
