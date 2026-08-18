package cli

import (
	"fmt"

	"github.com/devicelab-dev/maestro-runner/pkg/core"
	cdpdriver "github.com/devicelab-dev/maestro-runner/pkg/driver/browser/cdp"
	"github.com/devicelab-dev/maestro-runner/pkg/executor"
	"github.com/devicelab-dev/maestro-runner/pkg/logger"
)

// CreateWebDriver creates a browser driver using Rod + CDP.
// Exported for library use.
func CreateWebDriver(cfg *RunConfig) (core.Driver, func(), error) {
	driverConfig := buildWebDriverConfig(cfg)
	printSetupStep("Launching browser...")
	logger.Info("Creating web driver (headless=%v)", driverConfig.Headless)

	driver, err := cdpdriver.New(driverConfig)
	if err != nil {
		logger.Error("Failed to launch browser: %v", err)
		return nil, nil, fmt.Errorf("launch browser: %w", err)
	}

	printSetupSuccess("Browser launched")
	cleanup := func() {
		if err := driver.Close(); err != nil {
			logger.Debug("failed to close browser driver during cleanup: %v", err)
		}
	}
	return driver, cleanup, nil
}

// buildWebDriverConfig expands the flow header with the runner environment
// before the CDP driver's initial navigation.
func buildWebDriverConfig(cfg *RunConfig) cdpdriver.Config {
	script := executor.NewScriptEngine()
	defer script.Close()
	script.ImportSystemEnv()
	script.SetVariables(cfg.Env)

	return cdpdriver.Config{
		Headless:    !cfg.Headed,
		URL:         script.ExpandVariables(cfg.AppID),
		Browser:     cfg.Browser,
		UserDataDir: cfg.UserDataDir,
	}
}
