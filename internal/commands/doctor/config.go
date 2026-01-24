package doctor

import (
	"context"
	"errors"

	"github.com/hay-kot/criterio"
	"github.com/hay-kot/hive/internal/core/config"
)

// ConfigCheck validates the configuration file.
type ConfigCheck struct {
	config     *config.Config
	configPath string
}

// NewConfigCheck creates a new configuration check.
func NewConfigCheck(cfg *config.Config, configPath string) *ConfigCheck {
	return &ConfigCheck{
		config:     cfg,
		configPath: configPath,
	}
}

func (c *ConfigCheck) Name() string {
	return "Configuration"
}

func (c *ConfigCheck) Run(ctx context.Context) Result {
	result := Result{Name: c.Name()}

	if c.config == nil {
		result.Items = append(result.Items, CheckItem{
			Label:  "Config loaded",
			Status: StatusFail,
			Detail: "configuration not loaded",
		})
		return result
	}

	err := c.config.ValidateDeep(c.configPath)
	warnings := c.config.Warnings()

	// If no errors and no warnings, report success
	if err == nil && len(warnings) == 0 {
		result.Items = append(result.Items, CheckItem{
			Label:  "Config valid",
			Status: StatusPass,
		})
		return result
	}

	// Extract and report errors
	if err != nil {
		var fieldErrs criterio.FieldErrors
		if errors.As(err, &fieldErrs) {
			for _, fe := range fieldErrs {
				label := fe.Field
				if label == "" {
					label = "validation"
				}
				result.Items = append(result.Items, CheckItem{
					Label:  label,
					Status: StatusFail,
					Detail: fe.Err.Error(),
				})
			}
		} else {
			result.Items = append(result.Items, CheckItem{
				Label:  "validation",
				Status: StatusFail,
				Detail: err.Error(),
			})
		}
	}

	// Extract and report warnings
	for _, w := range warnings {
		label := w.Category
		if w.Item != "" {
			label += " (" + w.Item + ")"
		}
		result.Items = append(result.Items, CheckItem{
			Label:  label,
			Status: StatusWarn,
			Detail: w.Message,
		})
	}

	return result
}
