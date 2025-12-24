package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"

	"linyapsmanager/internal/dbusconsts"
	"linyapsmanager/internal/dbusutil"
	"linyapsmanager/internal/streaming"
)

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  linyapsctl version [--json]")
	fmt.Println("  linyapsctl get-version [--json]")
	fmt.Println("  linyapsctl help-remote")
	fmt.Println("  linyapsctl repo show [--json]")
	fmt.Println("  linyapsctl list [--json] [--upgradable] [--type=app|all]")
	fmt.Println("  linyapsctl list-upgradable [--json]           (alias)")
	fmt.Println("  linyapsctl list-upgradable-app [--json]       (alias)")
	fmt.Println("  linyapsctl search <keyword> [--json]")
	fmt.Println("  linyapsctl info <appId>")
	fmt.Println("  linyapsctl ps [--json]")
	fmt.Println("  linyapsctl install <appId>[/<version>] [--force] [-y]")
	fmt.Println("  linyapsctl uninstall <appId>[/<version>]")
	fmt.Println("  linyapsctl run <appId>[/<version>]")
	fmt.Println("  linyapsctl kill [-s <signal>] <appId>")
	fmt.Println("  linyapsctl prune")
	fmt.Println("  linyapsctl exec <container> -- <args...>")
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	conn, err := dbusutil.Connect("")
	if err != nil {
		log.Fatalf("connect bus failed: %v", err)
	}
	defer conn.Close()

	obj := conn.Object(dbusconsts.BusName, dbus.ObjectPath(dbusconsts.ObjectPath))

	globalJSON, stripped := parseGlobalJSON(os.Args[1:])
	if len(stripped) == 0 {
		printUsage()
		os.Exit(1)
	}
	cmd := stripped[0]
	args := stripped[1:]

	switch cmd {
	case "version", "--version":
		handleGetVersion(obj, args, globalJSON)
	case "get-version":
		handleGetVersion(obj, args, globalJSON)
	case "help-remote":
		handleHelpRemote(obj)
	case "repo":
		handleRepo(obj, args, globalJSON)
	case "repo-show":
		handleRepo(obj, args, globalJSON)
	case "list":
		handleList(obj, args, globalJSON)
	case "list-upgradable":
		handleList(obj, append([]string{"--upgradable"}, args...), globalJSON)
	case "list-upgradable-app":
		handleList(obj, append([]string{"--upgradable", "--type=app"}, args...), globalJSON)
	case "search":
		handleSearch(obj, args, globalJSON)
	case "info":
		handleInfo(obj, args)
	case "ps":
		handlePs(obj, args, globalJSON)
	case "install":
		handleInstall(conn, obj, args)
	case "uninstall":
		handleUninstall(obj, args)
	case "run":
		handleRun(obj, args)
	case "kill":
		handleKill(obj, args)
	case "prune":
		handlePrune(obj, args)
	case "exec":
		handleExec(obj, args)
	case "test":
		testStream(conn, obj)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Println("Unknown command:", cmd)
		printUsage()
		os.Exit(1)
	}
}

func callString(obj dbus.BusObject, method string, params ...interface{}) (string, error) {
	var out string
	call := obj.Call(dbusconsts.Interface+"."+method, 0, params...)
	if call.Err != nil {
		return "", call.Err
	}
	if err := call.Store(&out); err != nil {
		return "", err
	}
	return out, nil
}

func testStream(conn *dbus.Conn, obj dbus.BusObject) {
	err := callStringStreaming(conn, obj, "TestStream")
	if err != nil {
		log.Fatalf("TestStream failed: %v", err)
	}
}

// callStringStreaming calls a streaming D-Bus method and waits for output signals.
func callStringStreaming(conn *dbus.Conn, obj dbus.BusObject, method string, params ...interface{}) error {
	receiver, err := streaming.NewReceiver(conn)
	if err != nil {
		return fmt.Errorf("failed to create signal receiver: %w", err)
	}
	defer receiver.Stop()

	var operationID string
	call := obj.Call(dbusconsts.Interface+"."+method, 0, params...)
	if call.Err != nil {
		return call.Err
	}
	if err := call.Store(&operationID); err != nil {
		return fmt.Errorf("failed to get operation ID: %w", err)
	}

	exitCode, errorMsg := receiver.WaitForOperation(operationID, func(data string, isStderr bool) {
		if isStderr {
			fmt.Fprint(os.Stderr, data)
		} else {
			fmt.Print(data)
		}
	})

	if exitCode != 0 {
		if errorMsg != "" {
			return fmt.Errorf("command failed with exit code %d: %s", exitCode, errorMsg)
		}
		return fmt.Errorf("command failed with exit code %d", exitCode)
	}
	return nil
}

func parseGlobalJSON(args []string) (bool, []string) {
	json := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			json = true
		} else {
			rest = append(rest, a)
		}
	}
	return json, rest
}

func parseJSONFlag(args []string) (bool, []string) {
	json := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			json = true
		} else {
			rest = append(rest, a)
		}
	}
	return json, rest
}

func parseForceFlag(args []string) (bool, []string) {
	force := false
	var rest []string
	for _, a := range args {
		if a == "--force" {
			force = true
			continue
		}
		if a == "-y" || a == "--yes" {
			// ignore confirmation flags; server always runs with -y
			continue
		} else {
			rest = append(rest, a)
		}
	}
	return force, rest
}

func parseKillArgs(args []string) (string, string, error) {
	var appID string
	var signal string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-s" || a == "--signal":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing value for %s", a)
			}
			signal = args[i+1]
			i++
		case strings.HasPrefix(a, "--signal="):
			signal = strings.TrimPrefix(a, "--signal=")
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("unknown option: %s", a)
		default:
			if appID != "" {
				return "", "", fmt.Errorf("unexpected extra argument: %s", a)
			}
			appID = a
		}
	}

	if appID == "" {
		return "", "", fmt.Errorf("appId is required")
	}
	return appID, signal, nil
}

// parseAppRef splits appId/version to keep CLI usage identical to ll-cli.
func parseAppRef(ref string) (string, string, error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expect appId/version, got %q", ref)
	}
	return parts[0], parts[1], nil
}

// parseAppRefOptional accepts appId or appId/version; version may be empty.
func parseAppRefOptional(ref string) (string, string, error) {
	if strings.Contains(ref, "/") {
		return parseAppRef(ref)
	}
	if ref == "" {
		return "", "", fmt.Errorf("appId cannot be empty")
	}
	return ref, "", nil
}

func handleGetVersion(obj dbus.BusObject, args []string, jsonGlobal bool) {
	jsonFlag, _ := parseJSONFlag(args)
	jsonFlag = jsonFlag || jsonGlobal
	out, err := callString(obj, "GetVersion", jsonFlag)
	if err != nil {
		log.Fatalf("GetVersion failed: %v", err)
	}
	fmt.Print(out)
}

func handleRepo(obj dbus.BusObject, args []string, jsonGlobal bool) {
	jsonFlag, rest := parseJSONFlag(args)
	jsonFlag = jsonFlag || jsonGlobal
	if len(rest) > 0 && !(len(rest) == 1 && rest[0] == "show") {
		fmt.Println("Usage: linyapsctl repo show [--json]")
		os.Exit(1)
	}
	out, err := callString(obj, "RepoShow", jsonFlag)
	if err != nil {
		log.Fatalf("RepoShow failed: %v", err)
	}
	fmt.Print(out)
}

func handleHelpRemote(obj dbus.BusObject) {
	out, err := callString(obj, "Help")
	if err != nil {
		log.Fatalf("Help failed: %v", err)
	}
	fmt.Print(out)
}

func handleListAll(obj dbus.BusObject, args []string) {
	jsonFlag, _ := parseJSONFlag(args)
	out, err := callString(obj, "ListAll", jsonFlag)
	if err != nil {
		log.Fatalf("ListAll failed: %v", err)
	}
	fmt.Print(out)
}

func handleList(obj dbus.BusObject, args []string, jsonGlobal bool) {
	jsonFlag, rest := parseJSONFlag(args)
	jsonFlag = jsonFlag || jsonGlobal
	upgradable := false
	typ := ""
	for _, a := range rest {
		switch a {
		case "--upgradable":
			upgradable = true
		case "--type=app":
			typ = "app"
		case "--type=all":
			typ = "all"
		default:
			fmt.Println("Usage: linyapsctl list [--json] [--upgradable] [--type=app|all]")
			os.Exit(1)
		}
	}

	switch {
	case upgradable && typ == "app":
		out, err := callString(obj, "ListUpgradableApp", jsonFlag)
		if err != nil {
			log.Fatalf("ListUpgradableApp failed: %v", err)
		}
		fmt.Print(out)
	case upgradable:
		out, err := callString(obj, "ListUpgradable", jsonFlag)
		if err != nil {
			log.Fatalf("ListUpgradable failed: %v", err)
		}
		fmt.Print(out)
	default:
		out, err := callString(obj, "ListAll", jsonFlag)
		if err != nil {
			log.Fatalf("List failed: %v", err)
		}
		fmt.Print(out)
	}
}

func handleListUpgradable(obj dbus.BusObject, args []string) {
	jsonFlag, _ := parseJSONFlag(args)
	out, err := callString(obj, "ListUpgradable", jsonFlag)
	if err != nil {
		log.Fatalf("ListUpgradable failed: %v", err)
	}
	fmt.Print(out)
}

func handleListUpgradableApp(obj dbus.BusObject, args []string) {
	jsonFlag, _ := parseJSONFlag(args)
	out, err := callString(obj, "ListUpgradableApp", jsonFlag)
	if err != nil {
		log.Fatalf("ListUpgradableApp failed: %v", err)
	}
	fmt.Print(out)
}

func handleSearch(obj dbus.BusObject, args []string, jsonGlobal bool) {
	if len(args) == 0 {
		fmt.Println("Usage: linyapsctl search <keyword> [--json]")
		os.Exit(1)
	}
	jsonFlag, rest := parseJSONFlag(args)
	jsonFlag = jsonFlag || jsonGlobal
	if len(rest) == 0 {
		fmt.Println("Usage: linyapsctl search <keyword> [--json]")
		os.Exit(1)
	}
	keyword := strings.Join(rest, " ")

	out, err := callString(obj, "Search", keyword, jsonFlag)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}
	fmt.Print(out)
}

func handleInfo(obj dbus.BusObject, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: linyapsctl info <appId>")
		os.Exit(1)
	}
	appID := args[0]
	out, err := callString(obj, "Info", appID)
	if err != nil {
		log.Fatalf("Info failed: %v", err)
	}
	fmt.Print(out)
}

func handlePs(obj dbus.BusObject, args []string, jsonGlobal bool) {
	jsonFlag, _ := parseJSONFlag(args)
	jsonFlag = jsonFlag || jsonGlobal
	out, err := callString(obj, "Ps", jsonFlag)
	if err != nil {
		log.Fatalf("Ps failed: %v", err)
	}
	fmt.Print(out)
}

func handleInstall(conn *dbus.Conn, obj dbus.BusObject, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: linyapsctl install <appId>[/<version>] [--force]")
		os.Exit(1)
	}
	force, rest := parseForceFlag(args)
	if len(rest) != 1 {
		fmt.Println("Usage: linyapsctl install <appId>[/<version>] [--force]")
		os.Exit(1)
	}
	appID, version, err := parseAppRefOptional(rest[0])
	if err != nil {
		log.Fatal(err)
	}

	if err := callStringStreaming(conn, obj, "InstallStream", appID, version, force); err != nil {
		log.Fatalf("Install failed: %v", err)
	}
}

func handleUninstall(obj dbus.BusObject, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: linyapsctl uninstall <appId>[/<version>]")
		os.Exit(1)
	}
	appID, version, err := parseAppRefOptional(args[0])
	if err != nil {
		log.Fatal(err)
	}

	out, err := callString(obj, "Uninstall", appID, version)
	if err != nil {
		log.Fatalf("Uninstall failed: %v", err)
	}
	fmt.Print(out)
}

func handleRun(obj dbus.BusObject, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: linyapsctl run <appId>[/<version>]")
		os.Exit(1)
	}
	appID, version, err := parseAppRefOptional(args[0])
	if err != nil {
		log.Fatal(err)
	}

	out, err := callString(obj, "Run", appID, version)
	if err != nil {
		log.Fatalf("Run failed: %v", err)
	}
	fmt.Print(out)
}

func handleKill(obj dbus.BusObject, args []string) {
	appID, signal, err := parseKillArgs(args)
	if err != nil {
		fmt.Println("Usage: linyapsctl kill [-s <signal>] <appId>")
		os.Exit(1)
	}

	out, err := callString(obj, "Kill", appID, signal)
	if err != nil {
		log.Fatalf("Kill failed: %v", err)
	}
	fmt.Print(out)
}

func handlePrune(obj dbus.BusObject, args []string) {
	if len(args) != 0 {
		fmt.Println("Usage: linyapsctl prune")
		os.Exit(1)
	}
	out, err := callString(obj, "Prune")
	if err != nil {
		log.Fatalf("Prune failed: %v", err)
	}
	fmt.Print(out)
}

func handleExec(obj dbus.BusObject, args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: linyapsctl exec <container> -- <args...>")
		os.Exit(1)
	}
	container := args[0]
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 1 || sep == len(args)-1 {
		fmt.Println("Usage: linyapsctl exec <container> -- <args...>")
		os.Exit(1)
	}
	cmdArgs := args[sep+1:]
	out, err := callString(obj, "Exec", container, cmdArgs)
	if err != nil {
		log.Fatalf("Exec failed: %v", err)
	}
	fmt.Print(out)
}
