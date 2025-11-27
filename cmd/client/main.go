package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"

	"linyapsmanager/internal/dbusconsts"
	"linyapsmanager/internal/dbusutil"
)

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  linyapsctl version [--json]")
	fmt.Println("  linyapsctl get-version [--json]")
	fmt.Println("  linyapsctl help-remote")
	fmt.Println("  linyapsctl repo-show [--json]")
	fmt.Println("  linyapsctl list [--json]")
	fmt.Println("  linyapsctl list-upgradable [--json]")
	fmt.Println("  linyapsctl list-upgradable-app [--json]")
	fmt.Println("  linyapsctl search <keyword> [--json]")
	fmt.Println("  linyapsctl info <appId>")
	fmt.Println("  linyapsctl ps [--json]")
	fmt.Println("  linyapsctl install <appId>/<version> [--force]")
	fmt.Println("  linyapsctl uninstall <appId>/<version>")
	fmt.Println("  linyapsctl run <appId>[/<version>]")
	fmt.Println("  linyapsctl kill <appId>")
	fmt.Println("  linyapsctl prune")
	fmt.Println("  linyapsctl exec <container> -- <args...>")
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	busAddr := os.Getenv("LINYAPS_DBUS_ADDRESS")
	conn, err := dbusutil.Connect(busAddr)
	if err != nil {
		log.Fatalf("connect bus failed: %v", err)
	}
	defer conn.Close()

	obj := conn.Object(dbusconsts.BusName, dbus.ObjectPath(dbusconsts.ObjectPath))

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "version", "--version":
		handleGetVersion(obj, args)
	case "get-version":
		handleGetVersion(obj, args)
	case "help-remote":
		handleHelpRemote(obj)
	case "repo-show":
		handleRepoShow(obj, args)
	case "list":
		handleListAll(obj, args)
	case "list-upgradable":
		handleListUpgradable(obj, args)
	case "list-upgradable-app":
		handleListUpgradableApp(obj, args)
	case "search":
		handleSearch(obj, args)
	case "info":
		handleInfo(obj, args)
	case "ps":
		handlePs(obj, args)
	case "install":
		handleInstall(obj, args)
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
		} else {
			rest = append(rest, a)
		}
	}
	return force, rest
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

func handleGetVersion(obj dbus.BusObject, args []string) {
	jsonFlag, _ := parseJSONFlag(args)
	out, err := callString(obj, "GetVersion", jsonFlag)
	if err != nil {
		log.Fatalf("GetVersion failed: %v", err)
	}
	fmt.Print(out)
}

func handleRepoShow(obj dbus.BusObject, args []string) {
	jsonFlag, _ := parseJSONFlag(args)
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

func handleSearch(obj dbus.BusObject, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: linyapsctl search <keyword> [--json]")
		os.Exit(1)
	}
	jsonFlag, rest := parseJSONFlag(args)
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

func handlePs(obj dbus.BusObject, args []string) {
	jsonFlag, _ := parseJSONFlag(args)
	out, err := callString(obj, "Ps", jsonFlag)
	if err != nil {
		log.Fatalf("Ps failed: %v", err)
	}
	fmt.Print(out)
}

func handleInstall(obj dbus.BusObject, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: linyapsctl install <appId>/<version> [--force]")
		os.Exit(1)
	}
	force, rest := parseForceFlag(args)
	if len(rest) != 1 {
		fmt.Println("Usage: linyapsctl install <appId>/<version> [--force]")
		os.Exit(1)
	}
	appID, version, err := parseAppRef(rest[0])
	if err != nil {
		log.Fatal(err)
	}

	out, err := callString(obj, "Install", appID, version, force)
	if err != nil {
		log.Fatalf("Install failed: %v", err)
	}
	fmt.Print(out)
}

func handleUninstall(obj dbus.BusObject, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: linyapsctl uninstall <appId>/<version>")
		os.Exit(1)
	}
	appID, version, err := parseAppRef(args[0])
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
	if len(args) != 1 {
		fmt.Println("Usage: linyapsctl kill <appId>")
		os.Exit(1)
	}
	appID := args[0]

	out, err := callString(obj, "Kill", appID)
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
