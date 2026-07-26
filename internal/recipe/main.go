package recipe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fxManagerProject/cli-installer/internal/ui"
	_ "github.com/go-sql-driver/mysql"
)

func Installer(ctx ui.Context, recipeURL string, targetDir string, values map[string]string, artifact string) error {
	recipeYAML, err := fetchRecipeYAML(recipeURL)
	if err != nil {
		return fmt.Errorf("fetching recipe: %w", err)
	}

	artifactBuild, err := strconv.Atoi(artifact)
	if err != nil {
		return fmt.Errorf("failed to identify artifacts: %w", err)
	}

	parsedRecipe, err := ParseRecipe(recipeYAML, artifactBuild)
	if err != nil {
		return fmt.Errorf("parsing recipe: %w", err)
	}

	values["recipeName"] = parsedRecipe.Name
	values["recipeDescription"] = parsedRecipe.Description

	if parsedRecipe.RequireDBConfig {
		_, hasHost := values["dbHost"]
		if !hasHost {
			defaultDBName := strings.ToLower(parsedRecipe.Name)
			if defaultDBName == "" || defaultDBName == "unnamed" {
				defaultDBName = "fxserver"
			}

			// Trigger DB prompt via ctx.Ask(...)
			creds, confirmed, err := ui.PromptDatabaseCredentials(ctx, defaultDBName)
			if err != nil {
				return fmt.Errorf("prompting database configuration: %w", err)
			}
			if !confirmed {
				return fmt.Errorf("installation aborted by user during database configuration")
			}

			// Store user choices into values
			values["dbHost"] = creds.Host
			values["dbPort"] = creds.Port
			values["dbUsername"] = creds.Username
			values["dbPassword"] = creds.Password
			values["dbName"] = creds.Database
			values["dbConnectionString"] = fmt.Sprintf("mysql://%s:%s@%s:%s/%s", creds.Username, creds.Password, creds.Host, creds.Port, creds.Database)
		}
	}

	serverDetails, confirmed, err := ui.PromptRecipeDetails(ctx, strings.ToLower(parsedRecipe.Name))
	if err != nil {
		return fmt.Errorf("prompting server details: %w", err)
	}
	if !confirmed {
		return fmt.Errorf("installation aborted by user during details setup")
	}

	values["serverName"] = serverDetails.ServerName
	values["maxClients"] = serverDetails.MaxClients
	values["serverEndpoints"] = fmt.Sprintf("endpoint_add_tcp \"0.0.0.0:%s\"\nendpoint_add_udp \"0.0.0.0:%s\"", serverDetails.Port, serverDetails.Port)
	values["addPrincipalsMaster"] = "add_ace fxmanager.master group.admin # provides backwards compatibility\n# add_ace fxmanager.group.[slug] group.admin # replace [slug] with the name of your admin group (optional)"

	totalTasks := len(parsedRecipe.Tasks)
	titles := make([]string, totalTasks+1) // +1 for final validation step

	for i, task := range parsedRecipe.Tasks {
		titles[i] = formatTaskTitle(task)
	}
	titles[totalTasks] = "Validating server files & writing configuration"

	// Register subtasks with the UI runner
	ctx.SetSubTasks(titles)

	deployCtx := make(DeployerCtx)
	for k, v := range parsedRecipe.Variables {
		deployCtx[k] = v
	}
	for k, v := range values {
		deployCtx[k] = v
	}
	deployCtx["svLicense"] = values["cfxlicense"]
	deployCtx["deploymentID"] = fmt.Sprintf("deploy-%d", time.Now().Unix())

	// Ensure deployment path exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating target dir: %w", err)
	}

	for i, task := range parsedRecipe.Tasks {
		ctx.SubTaskStarted(i)

		action := GetString(task, "action")
		engineTask, ok := Engine[action]
		if !ok {
			err := fmt.Errorf("unknown action '%s'", action)
			ctx.SubTaskDone(i, err)
			return err
		}

		// Setup timeout context for this specific task
		timeout := time.Duration(engineTask.TimeoutSeconds) * time.Second
		taskCtx, cancel := context.WithTimeout(context.Background(), timeout)

		// Create progress reporter for tasks that support it (e.g. downloads)
		progressCB := func(ratio float64) {
			ctx.SubTaskProgress(i, ratio)
		}

		var runErr error
		if action == "download_file" {
			runErr = runDownloadTaskWithProgress(taskCtx, task, targetDir, deployCtx, progressCB)
		} else {
			runErr = engineTask.Run(taskCtx, task, targetDir, deployCtx)
		}
		cancel()

		if runErr != nil {
			ctx.SubTaskDone(i, runErr)
			markFailedDeploy(targetDir)
			return fmt.Errorf("task %d (%s) failed: %w", i+1, action, runErr)
		}

		ctx.SubTaskDone(i, nil)
	}

	// 6. Run Final Validation Subtask
	valIdx := totalTasks
	ctx.SubTaskStarted(valIdx)

	if err := validateAndFinalize(targetDir, deployCtx); err != nil {
		ctx.SubTaskDone(valIdx, err)
		markFailedDeploy(targetDir)
		return err
	}

	ctx.SubTaskDone(valIdx, nil)
	return nil
}

// formatTaskTitle translates recipe YAML tasks into clean UI display names
func formatTaskTitle(task RecipeTask) string {
	action := GetString(task, "action")
	switch action {
	case "download_file":
		if dest := GetString(task, "path"); dest != "" {
			return fmt.Sprintf("Downloading %s", filepath.Base(dest))
		}
		return "Downloading asset"
	case "download_github":
		src := GetString(task, "src")
		dest := GetString(task, "dest")

		return fmt.Sprintf("Downloading GitHub repo %s to %s", src, dest)
	case "ensure_dir":
		if path := GetString(task, "path"); path != "" {
			return fmt.Sprintf("Creating directory %s", path)
		}
		return "Creating directory"
	case "remove_path":
		if path := GetString(task, "path"); path != "" {
			return fmt.Sprintf("Cleaning up %s", filepath.Base(path))
		}
		return "Cleaning up path"
	case "move_path":
		src := GetString(task, "src")
		dest := GetString(task, "dest")
		return fmt.Sprintf("Moving %s to %s", filepath.Base(src), dest)
	case "write_file":
		if file := GetString(task, "file"); file != "" {
			return fmt.Sprintf("Writing %s", filepath.Base(file))
		}
		return "Writing file"
	case "replace_string":
		return "Applying variable replacements"
	case "connect_database":
		return "Connecting to database"
	case "query_database":
		if file := GetString(task, "file"); file != "" {
			return fmt.Sprintf("Executing SQL script (%s)", filepath.Base(file))
		}
		return "Executing database queries"
	case "unzip":
		if dest := GetString(task, "dest"); dest != "" {
			return fmt.Sprintf("Extracting archive to %s", dest)
		}
		return "Unpacking archive"
	case "load_vars":
		return "Loading dynamic variables"
	default:
		return fmt.Sprintf("Running %s task", action)
	}
}

// runDownloadTaskWithProgress wraps HTTP download to send progress ratios to the UI
func runDownloadTaskWithProgress(ctx context.Context, task RecipeTask, basePath string, deployCtx DeployerCtx, onProgress func(float64)) error {
	url := GetString(task, "url")
	targetPath := SafePath(basePath, GetString(task, "path"))

	os.MkdirAll(filepath.Dir(targetPath), 0755)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Wrap reader to report fractional download progress
	pw := &progressReader{
		reader:     resp.Body,
		total:      resp.ContentLength,
		onProgress: onProgress,
	}

	_, err = io.Copy(out, pw)
	return err
}

type progressReader struct {
	reader     io.Reader
	total      int64
	current    int64
	onProgress func(float64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	if pr.total > 0 && pr.onProgress != nil {
		pr.onProgress(float64(pr.current) / float64(pr.total))
	}
	return n, err
}

func validateAndFinalize(targetDir string, ctxVars DeployerCtx) error {
	// Verify resources directory and server.cfg exist
	if _, err := os.Stat(filepath.Join(targetDir, "resources")); os.IsNotExist(err) {
		return fmt.Errorf("recipe validation failed: 'resources' folder was not created")
	}
	if _, err := os.Stat(filepath.Join(targetDir, "server.cfg")); os.IsNotExist(err) {
		return fmt.Errorf("recipe validation failed: 'server.cfg' was not created")
	}

	// Substitute template variables in server.cfg
	replaceTask := RecipeTask{
		"action": "replace_string",
		"mode":   "all_vars",
		"file":   "./server.cfg",
	}
	return Engine["replace_string"].Run(context.Background(), replaceTask, targetDir, ctxVars)
}

func markFailedDeploy(targetDir string) {
	filePath := filepath.Join(targetDir, "_DEPLOY_FAILED_DO_NOT_USE")
	_ = os.WriteFile(filePath, []byte("This deploy has failed, please do not use these files."), 0644)
}

func fetchRecipeYAML(urlOrPath string) (string, error) {
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		resp, err := http.Get(urlOrPath)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}

	body, err := os.ReadFile(urlOrPath)

	return string(body), err
}
