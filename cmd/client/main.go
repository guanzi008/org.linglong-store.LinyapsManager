package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/godbus/dbus/v5"

	"linyapsmanager/internal/cmdwhitelist"
	"linyapsmanager/internal/dbusconsts"
	"linyapsmanager/internal/dbusutil"
	"linyapsmanager/internal/streaming"
)

func main() {
	log.SetFlags(0)

	// Get the command name from how we were invoked
	execPath := os.Args[0]
	cmdName := filepath.Base(execPath)

	// Handle special case: if invoked as the original binary name
	if cmdName == "linyaps-client" || cmdName == "linyapsctl" {
		printUsage()
		os.Exit(1)
	}

	// Check if command is allowed
	if !cmdwhitelist.IsAllowed(cmdName) {
		fmt.Fprintf(os.Stderr, "Error: command %q is not allowed\n", cmdName)
		os.Exit(1)
	}

	// Get command arguments (everything after program name)
	args := os.Args[1:]

	// Connect to D-Bus
	conn, err := dbusutil.Connect("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to D-Bus: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Execute command via D-Bus
	exitCode, err := executeCommand(conn, cmdName, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func printUsage() {
	fmt.Println("LinyapsManager Client")
	fmt.Println()
	fmt.Println("This program should be invoked via symlinks named after the command to execute.")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  ln -s linyaps-client ll-cli")
	fmt.Println("  ./ll-cli install com.example.app")
	fmt.Println()
	fmt.Println("Allowed commands:")
	for cmd := range cmdwhitelist.AllowedCommands {
		fmt.Printf("  - %s\n", cmd)
	}
}

func executeCommand(conn *dbus.Conn, command string, args []string) (int, error) {
	obj := conn.Object(dbusconsts.BusName, dbus.ObjectPath(dbusconsts.ObjectPath))

	// Set up signal receiver before making the call
	receiver, err := streaming.NewReceiver(conn)
	if err != nil {
		return -1, fmt.Errorf("failed to create signal receiver: %w", err)
	}
	defer receiver.Stop()

	// Call ExecuteCommand
	var operationID string
	err = obj.Call(dbusconsts.Interface+".ExecuteCommand", 0, command, args).Store(&operationID)
	if err != nil {
		return -1, fmt.Errorf("D-Bus call failed: %w", err)
	}

	// Wait for output and completion
	exitCode, errorMsg := receiver.WaitForOperation(operationID, func(data string, isStderr bool) {
		if isStderr {
			fmt.Fprint(os.Stderr, data)
		} else {
			fmt.Print(data)
		}
	})

	if errorMsg != "" {
		return exitCode, fmt.Errorf("command failed: %s", errorMsg)
	}

	return exitCode, nil
}
