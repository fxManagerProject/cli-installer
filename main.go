// A utility for setting up a FiveM/RedM server using fxManager
//
// Usage:
//
//	fxmanager-installer -dir ./myserver -license cfxk_XXXX... -recipe https://github.com/overextended/txAdminRecipe
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fxManagerProject/cli-installer/internal/cfgwriter"
	"github.com/fxManagerProject/cli-installer/internal/downloader"
	"github.com/fxManagerProject/cli-installer/internal/ghrelease"
	"github.com/fxManagerProject/cli-installer/internal/jgartifacts"
	"github.com/fxManagerProject/cli-installer/internal/layout"
	"github.com/fxManagerProject/cli-installer/internal/platform"
	"github.com/fxManagerProject/cli-installer/internal/recipe"
)

const (
	fxManagerOwner = "fxManagerProject"
	fxManagerRepo  = "fxManager"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nfxsetup: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dir       = flag.String("dir", ".", "target directory to setup the server into")
		osFlag    = flag.String("os", "", "target OS: windows or linux (default: autodetect current OS)")
		license   = flag.String("license", "", "CFX license key to inject into server.cfg (get one at https://keymaster.fivem.net)")
		recipeURL = flag.String("recipe", "", "GitHub repo URL for a txAdmin recipe")
	)
	flag.Parse()

	target, err := platform.ParseOverride(*osFlag)
	if err != nil {
		return err
	}

	root, err := filepath.Abs(*dir)
	if err != nil {
		return fmt.Errorf("resolving target directory %q: %w", *dir, err)
	}

	fmt.Printf("fxsetup - target: %s, OS: %s\n\n", root, target)

	paths, err := layout.Scaffold(root, target.String())
	if err != nil {
		return fmt.Errorf("scaffolding directories: %w", err)
	}

	// fxServer must be extracted first: installFxManager places the
	// fxManager game resource into fxServer/citizen/system_resources,
	// which doesn't exist until the FXServer artifact itself has been
	// downloaded and extracted.
	if err := installFxServer(target, paths); err != nil {
		return fmt.Errorf("installing fxServer artifact: %w", err)
	}

	if err := installFxManager(target, paths); err != nil {
		return fmt.Errorf("installing fxManager: %w", err)
	}

	step("Writing server.cfg")
	if err := cfgwriter.Write(paths.ServerCfgPath, cfgwriter.Options{License: *license}); err != nil {
		return fmt.Errorf("writing server.cfg: %w", err)
	}
	done()

	if *recipeURL != "" {
		step("Fetching recipe: " + *recipeURL)
		if err := recipe.Fetch(*recipeURL, paths.ServerDataDir); err != nil {
			return fmt.Errorf("fetching recipe: %w", err)
		}
		done()
	}

	fmt.Println("\nDone. Layout:")
	fmt.Printf("  %s\n", paths.Root)
	fmt.Printf("  %s\n", paths.FxServerDir)
	fmt.Printf("  %s\n", paths.ServerDataDir)
	if *license == "" {
		fmt.Println("\nNo --license passed - edit sv_licenseKey in server.cfg before starting the server.")
	}
	return nil
}

// installFxManager fetches the latest fxManager release and installs
// both halves of it: the webpanel binary+assets go into fxManager/,
// the FXServer-side lua resource goes into
// fxServer/citizen/system_resources/fxManager.
func installFxManager(target platform.Target, paths *layout.Paths) error {
	step("Resolving latest fxManager release")
	rel, err := ghrelease.Latest(fxManagerOwner, fxManagerRepo)
	if err != nil {
		return err
	}
	fmt.Printf(" -> %s\n", rel.TagName)

	panelAsset, resourceAsset, err := pickFxManagerAssets(rel, target)
	if err != nil {
		return err
	}

	step(fmt.Sprintf("Downloading + extracting webpanel (%s)", panelAsset.Name))
	if err := downloader.DownloadAndExtract(panelAsset.DownloadURL, paths.Root, panelAsset.Name, progress()); err != nil {
		return err
	}
	done()

	step(fmt.Sprintf("Downloading + extracting game resource (%s)", resourceAsset.Name))
	resourceTmp, err := downloader.DownloadAndExtractToTemp(resourceAsset.DownloadURL, resourceAsset.Name, progress())
	if err != nil {
		return err
	}
	defer os.RemoveAll(resourceTmp)
	done()

	step("Placing fxManager resource into citizen/system_resources")
	if err := paths.PlaceFxManagerResource(resourceTmp); err != nil {
		return err
	}
	done()

	step("Removing txAdmin (monitor) resource from citizen/system_resources")
	if err := paths.RemoveTxAdminResource(); err != nil {
		return err
	}
	done()

	return nil
}

// pickFxManagerAssets separates the OS-specific webpanel archive from
// the OS-independent game resource archive among a release's assets.
// Matching is done by substring rather than a fixed filename, since
// exact asset names have changed across fxManager releases before.
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

// installFxServer resolves the currently-recommended CFX artifact build
// from jgscripts and extracts it into fxServer/.
func installFxServer(target platform.Target, paths *layout.Paths) error {
	step("Resolving recommended FXServer artifact")
	url, label, err := jgartifacts.ResolveDownloadURL(target.String())
	if err != nil {
		return err
	}
	fmt.Printf(" -> build %s\n", label)

	step("Downloading + extracting FXServer artifact")
	if err := downloader.DownloadAndExtract(url, paths.FxServerDir, url, progress()); err != nil {
		return err
	}
	done()
	return nil
}

// console progress helpers

func step(msg string) {
	fmt.Printf("==> %s...", msg)
}

func done() {
	fmt.Println(" done")
}

func progress() *downloader.Progress {
	return &downloader.Progress{
		OnStart: func(url string) { fmt.Print(" downloading...") },
		OnDone:  func(url, destDir string) { fmt.Print(" extracted...") },
	}
}
