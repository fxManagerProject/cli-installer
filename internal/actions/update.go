package actions

import (
	"errors"
	"fmt"

	"github.com/fxManagerProject/cli-installer/internal/ui"
)

type UpdateTaskType string

const (
	UpdateFxManager UpdateTaskType = "update-fxmanager"
	UpdateFxServer  UpdateTaskType = "update-fxserver"
	UpdateAll       UpdateTaskType = "update-all"
)

var ErrUpdateNotImplemented = errors.New("updating fxManager via the CLI installer is not yet available")

func updateTasks(values map[string]string, taskType UpdateTaskType) []ui.Task {
	switch taskType {
	case UpdateFxManager:
		return []ui.Task{
			{
				Title: "Checking update availability",
				Run: func(ctx ui.Context) error {
					return ErrUpdateNotImplemented
				},
			},
		}
	case UpdateFxServer:
		return []ui.Task{
			{
				Title: "Checking update availability",
				Run: func(ctx ui.Context) error {
					return ErrUpdateNotImplemented
				},
			},
		}
	case UpdateAll:
		return []ui.Task{
			{
				Title: "Checking update availability",
				Run: func(ctx ui.Context) error {
					return ErrUpdateNotImplemented
				},
			},
		}
	default:
		return []ui.Task{
			{
				Title: fmt.Sprintf("Unknown update task: %s", taskType),
				Run: func(ctx ui.Context) error {
					return fmt.Errorf("unsupported update task %q", taskType)
				},
			},
		}
	}
}
