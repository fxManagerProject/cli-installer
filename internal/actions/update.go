package actions

import (
	"errors"

	"github.com/fxManagerProject/cli-installer/internal/ui"
)

var ErrUpdateNotImplemented = errors.New("updating fxManager via the CLI installer is not yet available")

func updateTasks(values map[string]string) []ui.Task {
	return []ui.Task{
		{
			Title: "Checking update availability",
			Run: func(ctx ui.Context) error {
				return ErrUpdateNotImplemented
			},
		},
	}
}
