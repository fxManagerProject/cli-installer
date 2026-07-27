package actions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fxManagerProject/cli-installer/internal/downloader"
	"github.com/fxManagerProject/cli-installer/internal/ghrelease"
	"github.com/fxManagerProject/cli-installer/internal/layout"
	"github.com/fxManagerProject/cli-installer/internal/platform"
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
		return buildFxManagerUpdateTasks(values)

	case UpdateFxServer:
		return buildFxServerUpdateTasks(values)

	case UpdateAll:
		var tasks []ui.Task
		tasks = append(tasks, buildFxManagerUpdateTasks(values)...)
		tasks = append(tasks, buildFxServerUpdateTasks(values)...)
		return tasks

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

// buildFxManagerUpdateTasks contains all fxManager update steps.
func buildFxManagerUpdateTasks(values map[string]string) []ui.Task {
	var (
		target          platform.Target
		paths           *layout.Paths
		latestRel       *ghrelease.Release
		panelAsset      *ghrelease.Asset
		resourceAsset   *ghrelease.Asset
		sysResourcesTmp string
		shouldUpdate    = true
	)

	return []ui.Task{
		{
			Title: "Scaffolding directory environment",
			Run: func(ctx ui.Context) error {
				var err error
				target, err = platform.ParseOverride(values["os"])
				if err != nil {
					return err
				}

				dir := values["dir"]
				if dir == "" {
					dir = "."
				}

				root, err := filepath.Abs(dir)
				if err != nil {
					return fmt.Errorf("resolving target directory %q: %w", dir, err)
				}

				paths, err = layout.Scaffold(root, target.String())
				if err != nil {
					return fmt.Errorf("scaffolding directories: %w", err)
				}
				return nil
			},
		},
		{
			Title: "Checking webpanel version",
			Run: func(ctx ui.Context) error {
				rel, err := ghrelease.Latest(fxManagerOwner, fxManagerRepo)
				if err != nil {
					return fmt.Errorf("checking latest webpanel release: %w", err)
				}
				latestRel = rel

				versionPath := filepath.Join(paths.Root, "VERSION")
				data, err := os.ReadFile(versionPath)

				if err == nil {
					installedVersion := strings.TrimSpace(string(data))

					if IsSameOrNewer(installedVersion, rel.TagName) {
						reinstall, err := ui.PromptReinstallWebpanel(ctx, installedVersion, rel.TagName)
						if err != nil {
							return err
						}
						if !reinstall {
							shouldUpdate = false
							return nil
						}
					}
				}

				var errAsset error
				panelAsset, resourceAsset, errAsset = pickFxManagerAssets(rel, target)
				if errAsset != nil {
					return errAsset
				}

				return nil
			},
		},
		{
			Title: "Updating fxManager webpanel",
			Run: func(ctx ui.Context) error {
				if !shouldUpdate {
					return nil
				}

				webpanelDirsToClean := []string{"assets", panelAsset.Name}
				for _, dir := range webpanelDirsToClean {
					_ = os.RemoveAll(filepath.Join(paths.Root, dir))
				}

				prog := &downloader.Progress{
					OnProgress: func(ratio float64) {
						ctx.Report(ratio)
					},
				}

				if err := downloader.DownloadAndExtract(panelAsset.DownloadURL, paths.Root, panelAsset.Name, prog); err != nil {
					return fmt.Errorf("downloading webpanel: %w", err)
				}

				versionPath := filepath.Join(paths.Root, "VERSION")
				if err := os.WriteFile(versionPath, []byte(latestRel.TagName), 0644); err != nil {
					return fmt.Errorf("writing VERSION file: %w", err)
				}

				return nil
			},
		},
		{
			Title: "Downloading fxManager game resource",
			Run: func(ctx ui.Context) error {
				prog := &downloader.Progress{
					OnProgress: func(ratio float64) {
						ctx.Report(ratio)
					},
				}

				var err error
				sysResourcesTmp, err = downloader.DownloadAndExtractToTemp(resourceAsset.DownloadURL, resourceAsset.Name, prog)
				if err != nil {
					return fmt.Errorf("downloading game resource: %w", err)
				}
				return nil
			},
		},
		{
			Title:         "Moving game resource into system_resources",
			Indeterminate: true,
			Run: func(ctx ui.Context) error {
				if !shouldUpdate {
					return nil
				}

				defer os.RemoveAll(sysResourcesTmp)

				targetResourceDir := filepath.Join(paths.SystemResDir, "fxManager")
				if err := os.RemoveAll(targetResourceDir); err != nil {
					return fmt.Errorf("clearing old game resource directory: %w", err)
				}

				if err := paths.PlaceFxManagerResource(sysResourcesTmp); err != nil {
					return err
				}
				return nil
			},
		},
	}
}

// buildFxServerUpdateTasks contains FXServer update steps.
func buildFxServerUpdateTasks(values map[string]string) []ui.Task {

	return []ui.Task{
		{
			Title: "Checking update availability",
			Run: func(ctx ui.Context) error {
				return ErrUpdateNotImplemented
			},
		},
	}
}
