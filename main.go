package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

// Define styles using Lip Gloss v2
var (
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	doneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("84"))
	subTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("197")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// Bubble Tea Messages
type statusMsg string
type subStatusMsg string
type progressMsg float64
type hideProgressMsg struct{}
type errMsg struct{ err error }
type successMsg struct {
	paths     *layout.Paths
	noLicense bool
}

// Bubble Tea Model
type model struct {
	spinner      spinner.Model
	progress     progress.Model
	showProgress bool
	steps        []string
	activeStep   string
	subStatus    string
	err          error
	complete     bool
	finalPaths   *layout.Paths
	noLicense    bool
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		}
	case statusMsg:
		if m.activeStep != "" {
			m.steps = append(m.steps, "✔ "+m.activeStep)
		}
		m.activeStep = string(msg)
		m.subStatus = ""
		m.showProgress = false // Default to spinner on new steps
		return m, nil
	case subStatusMsg:
		m.subStatus = string(msg)
		return m, nil
	case progressMsg:
		m.showProgress = true
		return m, m.progress.SetPercent(float64(msg))
	case hideProgressMsg:
		m.showProgress = false
		return m, nil
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	case errMsg:
		m.err = msg.err
		return m, tea.Quit
	case successMsg:
		if m.activeStep != "" {
			m.steps = append(m.steps, "✔ "+m.activeStep)
		}
		m.complete = true
		m.finalPaths = msg.paths
		m.noLicense = msg.noLicense
		return m, tea.Quit
	default:
		// Route generic messages (like window resize) and internal ticks to both components
		var spinnerCmd, progressCmd tea.Cmd

		m.spinner, spinnerCmd = m.spinner.Update(msg)
		m.progress, progressCmd = m.progress.Update(msg)

		return m, tea.Batch(spinnerCmd, progressCmd)
	}

	return m, nil
}

func (m model) View() tea.View {
	var s strings.Builder

	fmt.Fprintf(&s, "\n  %s\n", titleStyle.Render("fxManager Server Installer"))
	s.WriteString("  =============================\n\n")

	for _, step := range m.steps {
		fmt.Fprintf(&s, "  %s\n", doneStyle.Render(step))
	}

	if m.err != nil {
		fmt.Fprintf(&s, "\n  %s\n", errorStyle.Render("❌ Error: "+m.err.Error()))
		return tea.NewView(s.String())
	}

	if m.complete {
		fmt.Fprintf(&s, "\n  %s\n\n", doneStyle.Render("🎉 Installation Successfully Completed!"))
		s.WriteString("  Directories Layout:\n")
		fmt.Fprintf(&s, "    Root:        %s\n", m.finalPaths.Root)
		fmt.Fprintf(&s, "    FXServer:    %s\n", m.finalPaths.FxServerDir)
		fmt.Fprintf(&s, "    ServerData:  %s\n", m.finalPaths.ServerDataDir)
		if m.noLicense {
			fmt.Fprintf(&s, "\n  %s\n", warnStyle.Render("⚠️ Warning: No --license passed. Edit server.cfg before starting."))
		}
		return tea.NewView(s.String())
	}

	if m.activeStep != "" {
		sub := ""
		if m.subStatus != "" {
			sub = subTextStyle.Render(" (" + m.subStatus + ")")
		}

		// Toggle between rendering the progress bar or the spinner
		if m.showProgress {
			fmt.Fprintf(&s, "  %s %s%s\n", m.progress.View(), m.activeStep, sub)
		} else {
			fmt.Fprintf(&s, "  %s %s%s\n", m.spinner.View(), m.activeStep, sub)
		}
	}

	s.WriteString(subTextStyle.Render("\n  [Press Q or Ctrl+C to abort]\n"))
	return tea.NewView(s.String())
}

// buildProgress is a helper to avoid repetitive callback declarations
func buildProgress(p *tea.Program, startMsg, doneMsg string) *downloader.Progress {
	return &downloader.Progress{
		OnStart: func(url string) {
			p.Send(subStatusMsg(startMsg))
		},
		OnProgress: func(ratio float64) {
			p.Send(progressMsg(ratio))
		},
		OnDone: func(url, destDir string) {
			p.Send(hideProgressMsg{}) // Switch back to the spinner for extraction
			p.Send(subStatusMsg(doneMsg))
		},
	}
}

func main() {
	var (
		dir       = flag.String("dir", ".", "target directory to setup the server into")
		osFlag    = flag.String("os", "", "target OS: windows or linux (default: autodetect current OS)")
		license   = flag.String("license", "", "CFX license key to inject into server.cfg (get one at https://keymaster.fivem.net)")
		recipeURL = flag.String("recipe", "", "GitHub repo URL for a txAdmin recipe")
	)
	flag.Parse()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))

	prog := progress.New(progress.WithDefaultBlend())

	m := model{
		spinner:  sp,
		progress: prog,
	}
	p := tea.NewProgram(m)

	go runInstallationPipeline(p, *dir, *osFlag, *license, *recipeURL)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal UI error: %v\n", err)
		os.Exit(1)
	}
}

// Background worker pipeline
func runInstallationPipeline(p *tea.Program, dir, osFlag, license, recipeURL string) {
	target, err := platform.ParseOverride(osFlag)
	if err != nil {
		p.Send(errMsg{err})
		return
	}

	root, err := filepath.Abs(dir)
	if err != nil {
		p.Send(errMsg{fmt.Errorf("resolving target directory %q: %w", dir, err)})
		return
	}

	p.Send(statusMsg("Scaffolding directory environment"))
	paths, err := layout.Scaffold(root, target.String())
	if err != nil {
		p.Send(errMsg{fmt.Errorf("scaffolding directories: %w", err)})
		return
	}

	p.Send(statusMsg("Resolving recommended FXServer artifact"))
	url, label, err := jgartifacts.ResolveDownloadURL(target.String())
	if err != nil {
		p.Send(errMsg{err})
		return
	}

	p.Send(statusMsg(fmt.Sprintf("Downloading FXServer artifact (build %s)", label)))
	progServer := buildProgress(p, "downloading...", "extracting...")
	if err := downloader.DownloadAndExtract(url, paths.FxServerDir, url, progServer); err != nil {
		p.Send(errMsg{fmt.Errorf("installing fxServer artifact: %w", err)})
		return
	}

	p.Send(statusMsg("Resolving latest fxManager GitHub release"))
	rel, err := ghrelease.Latest(fxManagerOwner, fxManagerRepo)
	if err != nil {
		p.Send(errMsg{err})
		return
	}

	panelAsset, resourceAsset, err := pickFxManagerAssets(rel, target)
	if err != nil {
		p.Send(errMsg{err})
		return
	}

	p.Send(statusMsg("Downloading fxManager webpanel"))
	progPanel := buildProgress(p, "fetching archive...", "extracting webpanel...")
	if err := downloader.DownloadAndExtract(panelAsset.DownloadURL, paths.Root, panelAsset.Name, progPanel); err != nil {
		p.Send(errMsg{err})
		return
	}

	p.Send(statusMsg("Downloading fxManager lua game resource"))
	progRes := buildProgress(p, "fetching archive...", "unpacking to temp...")
	resourceTmp, err := downloader.DownloadAndExtractToTemp(resourceAsset.DownloadURL, resourceAsset.Name, progRes)
	if err != nil {
		p.Send(errMsg{err})
		return
	}
	defer os.RemoveAll(resourceTmp)

	p.Send(statusMsg("Moving game resource into system_resources"))
	if err := paths.PlaceFxManagerResource(resourceTmp); err != nil {
		p.Send(errMsg{err})
		return
	}

	p.Send(statusMsg("Clearing out stock txAdmin monitor resource"))
	if err := paths.RemoveTxAdminResource(); err != nil {
		p.Send(errMsg{err})
		return
	}

	p.Send(statusMsg("Writing server configuration (server.cfg)"))
	if err := cfgwriter.Write(paths.ServerCfgPath, cfgwriter.Options{License: license}); err != nil {
		p.Send(errMsg{fmt.Errorf("writing server.cfg: %w", err)})
		return
	}

	if recipeURL != "" {
		p.Send(statusMsg("Evaluating deployment recipe setup"))
		if err := recipe.Fetch(recipeURL, paths.ServerDataDir); err != nil {
			p.Send(errMsg{fmt.Errorf("fetching recipe: %w", err)})
			return
		}
	}

	p.Send(successMsg{paths: paths, noLicense: license == ""})
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
