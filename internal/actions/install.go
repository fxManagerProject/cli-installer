package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fxManagerProject/cli-installer/internal/artifacts"
	"github.com/fxManagerProject/cli-installer/internal/cfgwriter"
	"github.com/fxManagerProject/cli-installer/internal/downloader"
	"github.com/fxManagerProject/cli-installer/internal/ghrelease"
	"github.com/fxManagerProject/cli-installer/internal/layout"
	"github.com/fxManagerProject/cli-installer/internal/platform"
	"github.com/fxManagerProject/cli-installer/internal/recipe"
	"github.com/fxManagerProject/cli-installer/internal/ui"
)

const (
	fxManagerOwner = "fxManagerProject"
	fxManagerRepo  = "fxManager"
)

var recipeURLs = map[string]string{
	"fivem-default": "https://raw.githubusercontent.com/citizenfx/txAdmin-recipes/refs/heads/main/default-fivem/recipe.yaml",
	"ox_core":       "https://raw.githubusercontent.com/overextended/txAdminRecipe/main/recipe.yaml",
	"qbox":          "https://raw.githubusercontent.com/Qbox-project/txAdminRecipe/refs/heads/main/qbox.yaml",
	"esx":           "https://raw.githubusercontent.com/esx-framework/ESX-recipes/legacy/recipe.yaml",
	"redm-default":  "https://raw.githubusercontent.com/citizenfx/txAdmin-recipes/refs/heads/main/default-redm/recipe.yaml",
	"vorp":          "https://raw.githubusercontent.com/VORPCORE/VORP_txAdmin/main/vorp_recipe.yaml",
}

// installTasks constructs the step-by-step task pipeline for a fresh installation.
func installTasks(values map[string]string) []ui.Task {
	// Shared pipeline state passed down sequentially between task closures
	var (
		target          platform.Target
		paths           *layout.Paths
		panelAsset      *ghrelease.Asset
		resourceAsset   *ghrelease.Asset
		sysResourcesTmp string
		artifact        string
	)

	tasks := []ui.Task{
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
			Title: "Downloading FXServer artifact",
			Run: func(ctx ui.Context) error {
				res, err := artifacts.Resolve(target.String(), values["artifact"])
				if err != nil {
					return err
				}

				if res.IsBroken {
					confirmed, err := ui.PromptBrokenArtifact(ctx, res.ArtifactLabel, res.BrokenReason)
					if err != nil {
						return err
					}
					if !confirmed {
						return fmt.Errorf("installation aborted by user: artifact build %s is broken (%s)", res.ArtifactLabel, res.BrokenReason)
					}
				}

				artifact = res.ArtifactLabel

				prog := &downloader.Progress{
					OnProgress: func(ratio float64) {
						ctx.Report(ratio)
					},
				}

				if err := downloader.DownloadAndExtract(res.URL, paths.FxServerDir, res.URL, prog); err != nil {
					return fmt.Errorf("installing fxServer artifact (build %s): %w", res.ArtifactLabel, err)
				}
				return nil
			},
		},
		{
			Title: "Downloading fxManager webpanel",
			Run: func(ctx ui.Context) error {
				rel, err := ghrelease.Latest(fxManagerOwner, fxManagerRepo)
				if err != nil {
					return err
				}

				var errAsset error
				panelAsset, resourceAsset, errAsset = pickFxManagerAssets(rel, target)
				if errAsset != nil {
					return errAsset
				}

				prog := &downloader.Progress{
					OnProgress: func(ratio float64) {
						ctx.Report(ratio)
					},
				}

				if err := downloader.DownloadAndExtract(panelAsset.DownloadURL, paths.Root, panelAsset.Name, prog); err != nil {
					return fmt.Errorf("downloading webpanel: %w", err)
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
				defer os.RemoveAll(sysResourcesTmp)
				if err := paths.PlaceFxManagerResource(sysResourcesTmp); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Title:         "Clearing out stock txAdmin monitor resource",
			Indeterminate: true,
			Run: func(ctx ui.Context) error {
				if err := paths.RemoveTxAdminResource(); err != nil {
					return err
				}
				return nil
			},
		},
	}

	if recipeURL, exists := recipeURLs[values["recipe"]]; exists && values["recipe"] != "none" {
		tasks = append(tasks, ui.Task{
			Title: "Installing recipe: " + values["recipe"],
			Run: func(ctx ui.Context) error {
				if err := recipe.Installer(ctx, recipeURL, paths.ServerDataDir, values, artifact); err != nil {
					return fmt.Errorf("recipe installation failed: %w", err)
				}
				return nil
			},
		})
	} else {
		tasks = append(tasks, ui.Task{
			Title:         "Writing server configuration (server.cfg)",
			Indeterminate: true,
			Run: func(ctx ui.Context) error {
				license := values["cfxlicense"]
				if err := cfgwriter.Write(paths.ServerCfgPath, cfgwriter.Options{License: license}); err != nil {
					return fmt.Errorf("writing server.cfg: %w", err)
				}
				return nil
			},
		})
	}

	return tasks
}

func pickFxManagerAssets(rel *ghrelease.Release, target platform.Target) (panel, resource *ghrelease.Asset, err error) {
	osToken := strings.ToLower(target.FxManagerAssetPattern())
	resourceToken := strings.ToLower(platform.FxManagerResourcePattern())

	for i := range rel.Assets {
		name := strings.ToLower(rel.Assets[i].Name)
		switch {
		case strings.Contains(name, resourceToken):
			resource = &rel.Assets[i]
		case strings.Contains(name, osToken):
			panel = &rel.Assets[i]
		}
	}

	var missing []string
	if panel == nil {
		missing = append(missing, fmt.Sprintf("webpanel asset matching %q", osToken))
	}
	if resource == nil {
		missing = append(missing, fmt.Sprintf("resource asset matching %q", resourceToken))
	}
	if len(missing) > 0 {
		names := make([]string, len(rel.Assets))
		for i, a := range rel.Assets {
			names[i] = a.Name
		}
		return nil, nil, fmt.Errorf("release %s: could not find %s (available assets: %v)", rel.TagName, strings.Join(missing, ", "), names)
	}
	return panel, resource, nil
}
