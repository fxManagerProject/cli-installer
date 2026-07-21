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
	case "update-fxmanager":
		return updateTasks(values, UpdateFxManager)
	case "update-fxserver":
		return updateTasks(values, UpdateFxServer)
	case "update-all":
		return updateTasks(values, UpdateAll)
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
