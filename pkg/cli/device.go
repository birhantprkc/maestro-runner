package cli

import (
	"errors"
	"fmt"

	"github.com/devicelab-dev/maestro-runner/pkg/device"
	"github.com/devicelab-dev/maestro-runner/pkg/logger"
	"github.com/urfave/cli/v2"
)

var startDeviceCommand = &cli.Command{
	Name:  "start-device",
	Usage: "Start or create an iOS Simulator or Android Emulator",
	Description: `Start or create a device similar to ones used in cloud testing.
Requires --platform global flag (before command).

Examples:
  maestro-runner -p ios start-device --os-version 17
  maestro-runner -p android start-device --os-version 33
  maestro-runner -p ios start-device --device-locale de_DE`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "os-version",
			Usage: "OS version (iOS: 16, 17, 18; Android: 28-33)",
		},
		&cli.StringFlag{
			Name:  "device-locale",
			Usage: "Device locale (e.g., de_DE)",
		},
		&cli.BoolFlag{
			Name:  "force-create",
			Usage: "Override existing device",
		},
	},
	Action: runStartDevice,
}

var hierarchyCommand = &cli.Command{
	Name:  "hierarchy",
	Usage: "Print the view hierarchy of the connected device",
	Description: `Print out the view hierarchy of the connected device in the device-specific tree format

Examples:
  maestro-runner hierarchy
  maestro-runner hierarchy --device emulator-5554`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "compact",
			Usage: "Output in CSV format",
		},
	},
	Action: runHierarchy,
}

func runStartDevice(c *cli.Context) error {
	platform := c.String("platform") // Global flag
	if platform == "" {
		return fmt.Errorf("--platform is required (ios or android)")
	}

	osVersion := c.String("os-version")
	locale := c.String("device-locale")
	forceCreate := c.Bool("force-create")

	// TODO: Implement device creation
	fmt.Println("Start device command received:")
	fmt.Printf("  Platform: %s\n", platform)
	if osVersion != "" {
		fmt.Printf("  OS Version: %s\n", osVersion)
	}
	if locale != "" {
		fmt.Printf("  Locale: %s\n", locale)
	}
	if forceCreate {
		fmt.Println("  Force create: true")
	}

	fmt.Println("\n[Not yet implemented - will create/start device]")
	return nil
}

func runHierarchy(c *cli.Context) error {
	runDevice := c.String("device")
	compact := c.Bool("compact")

	fmt.Println("Hierarchy command received:")
	if runDevice != "" {
		fmt.Printf("  Device: %s\n", runDevice)
	}
	fmt.Println("\n[WARNING: Not yet fully tested - use with caution]")
	if compact {
		fmt.Println("[Compact mode not yet implemented, will print full hierarchy]")
	}

	// Helper to get flag value from current or parent context
	// When run as subcommand, global flags are in parent context
	// NOTE: This are duplicated from pkg/cli/test.go, may want to refactor
	getString := func(name string) string {
		if c.IsSet(name) {
			return c.String(name)
		}
		if c.Lineage()[1] != nil {
			return c.Lineage()[1].String(name)
		}
		return c.String(name)
	}
	getInt := func(name string) int {
		if c.IsSet(name) {
			return c.Int(name)
		}
		if c.Lineage()[1] != nil {
			return c.Lineage()[1].Int(name)
		}
		return c.Int(name)
	}
	getBool := func(name string) bool {
		if c.IsSet(name) {
			return c.Bool(name)
		}
		if c.Lineage()[1] != nil {
			return c.Lineage()[1].Bool(name)
		}
		return c.Bool(name)
	}

	// Load Appium capabilities if provided
	capsFile := getString("caps")
	var caps map[string]interface{}
	if capsFile != "" {
		var err error
		caps, err = loadCapabilities(capsFile)
		if err != nil {
			return err
		}
	}

	// Build run configuration, limited to elements relevant to hierarchy subcommand
	cfg := &RunConfig{
		Headed:             getBool("headed"),
		Browser:            getString("browser"),
		UserDataDir:        getString("user-data-dir"),
		Platform:           getString("platform"),
		Devices:            parseDevices(getString("device")),
		Driver:             getString("driver"),
		AppiumURL:          getString("appium-url"),
		AppiumSessionFile:  getString("appium-session-file"),
		CapsFile:           capsFile,
		Capabilities:       caps,
		TeamID:             getString("team-id"),
		WDABundleID:        getString("wda-bundle-id"),
		StartEmulator:      getString("start-emulator"),
		StartSimulator:     getString("start-simulator"),
		AutoStartEmulator:  getBool("auto-start-emulator"),
		BootTimeout:        getInt("boot-timeout"),
		DriverStartTimeout: getInt("driver-start-timeout"),
		NoDriverInstall:    getBool("no-driver-install"),
		NoFlutterFallback:  getBool("no-flutter-fallback"),
		AndroidTCPForward:  getBool("android-tcp-forward"),
	}

	driver, cleanup, err := CreateDriver(cfg)
	if err != nil {
		logger.Error("Failed to create driver: %v", err)
		// Surface NoDevicesError directly so the helpful message isn't buried
		var noDevErr *device.NoDevicesError
		if errors.As(err, &noDevErr) {
			logger.Error("NoDevicesError: %v", noDevErr)
			return nil
		}
		return nil
	}
	defer cleanup()

	tree, err := driver.Hierarchy()
	if err != nil {
		logger.Error("Failed to get hierarchy: %v", err)
		return nil
	}
	// TODO: If 'tree' is a JSON object and 'compact' is true, print the object as a CSV
	fmt.Println(string(tree))
	return nil
}
