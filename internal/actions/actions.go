package actions

import (
	"fmt"

	"github.com/fxManagerProject/cli-installer/internal/ui"
)

// Build inspects the resolved "action" parameter and returns the corresponding task pipeline.
func Build(values map[string]string) []ui.Task {
	action := values["action"]

	switch action {
	case "install":
		return installTasks(values)
	case "update":
		return updateTasks(values)
	default:
		return []ui.Task{
			{
				Title: fmt.Sprintf("Unknown action: %s", action),
				Run: func(ctx ui.Context) error {
					return fmt.Errorf("unsupported action %q", action)
				},
			},
		}
	}
}
