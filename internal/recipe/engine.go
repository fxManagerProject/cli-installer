package recipe

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type TaskDefinition struct {
	Validate       func(options RecipeTask) bool
	Run            func(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error
	TimeoutSeconds int
}

type ghRepoResponse struct {
	DefaultBranch string `json:"default_branch"`
}

var githubRepoRegex = regexp.MustCompile(`^((https?://github\.com/)?|@)?([\w.\-_]+)/([\w.\-_]+).*$`)

var Engine = map[string]TaskDefinition{
	"download_file": {
		Validate: func(options RecipeTask) bool {
			return GetString(options, "url") != "" && GetString(options, "path") != ""
		},
		Run:            taskDownloadFile,
		TimeoutSeconds: 180,
	},
	"download_github": {
		Validate: func(options RecipeTask) bool {
			src := GetString(options, "src")
			dest := GetString(options, "dest")
			return src != "" && dest != ""
		},
		Run:            taskDownloadGithub,
		TimeoutSeconds: 300,
	},
	"ensure_dir": {
		Validate: func(options RecipeTask) bool {
			return GetString(options, "path") != ""
		},
		Run:            taskEnsureDir,
		TimeoutSeconds: 15,
	},
	"remove_path": {
		Validate:       func(options RecipeTask) bool { return GetString(options, "path") != "" },
		Run:            taskRemovePath,
		TimeoutSeconds: 15,
	},
	"move_path": {
		Validate: func(options RecipeTask) bool {
			return GetString(options, "src") != "" && GetString(options, "dest") != ""
		},
		Run:            taskMovePath,
		TimeoutSeconds: 60,
	},
	"write_file": {
		Validate: func(options RecipeTask) bool {
			return GetString(options, "file") != "" && GetString(options, "data") != ""
		},
		Run:            taskWriteFile,
		TimeoutSeconds: 15,
	},
	"replace_string": {
		Validate: func(options RecipeTask) bool {
			// Basic validation
			if GetString(options, "file") == "" && options["file"] == nil {
				return false
			}
			mode := GetString(options, "mode")
			if mode == "" || mode == "template" || mode == "literal" {
				return GetString(options, "search") != "" && options["replace"] != nil
			} else if mode == "all_vars" {
				return true
			}
			return false
		},
		Run:            taskReplaceString,
		TimeoutSeconds: 15,
	},
	"connect_database": {
		Validate:       func(options RecipeTask) bool { return true },
		Run:            taskConnectDatabase,
		TimeoutSeconds: 30,
	},
	"query_database": {
		Validate: func(options RecipeTask) bool {
			hasFile := GetString(options, "file") != ""
			hasQuery := GetString(options, "query") != ""
			return hasFile != hasQuery // XOR
		},
		Run:            taskQueryDatabase,
		TimeoutSeconds: 90,
	},
	"unzip": {
		Validate: func(options RecipeTask) bool {
			return GetString(options, "src") != "" && GetString(options, "dest") != ""
		},
		Run:            taskUnzip,
		TimeoutSeconds: 180,
	},
	"load_vars": {
		Validate:       func(options RecipeTask) bool { return GetString(options, "src") != "" },
		Run:            taskLoadVars,
		TimeoutSeconds: 5,
	},
	"waste_time": {
		Validate: func(options RecipeTask) bool { return options["seconds"] != nil },
		Run: func(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
			secs := options["seconds"].(int)
			time.Sleep(time.Duration(secs) * time.Second)
			return nil
		},
		TimeoutSeconds: 300,
	},
}

func taskDownloadFile(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	url := GetString(options, "url")
	targetPath := SafePath(basePath, GetString(options, "path"))

	if strings.HasSuffix(GetString(options, "path"), "/") {
		return errors.New("target filename not specified")
	}

	// Ensure dir exists
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

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func taskDownloadGithub(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	src := GetString(options, "src")
	dest := GetString(options, "dest")
	ref := GetString(options, "ref")
	subpath := GetString(options, "subpath")

	matches := githubRepoRegex.FindStringSubmatch(src)
	if len(matches) < 5 || matches[3] == "" || matches[4] == "" {
		return errors.New("invalid github repository source format")
	}
	repoOwner := matches[3]
	repoName := matches[4]

	deployCtx["$step"] = "ref set"

	// Resolve Git Reference (branch/tag) if not provided
	if ref == "" {
		apiUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s", repoOwner, repoName)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "txAdmin-Deployer-Go")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("fetching github repo info: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("github api returned status %d", resp.StatusCode)
		}

		var ghData ghRepoResponse
		if err := json.NewDecoder(resp.Body).Decode(&ghData); err != nil {
			return fmt.Errorf("parsing github repo json: %w", err)
		}
		if ghData.DefaultBranch == "" {
			return errors.New("reference not set, and was unable to detect default_branch from GitHub API")
		}
		ref = ghData.DefaultBranch
	}

	//  temporary file download path
	downURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/zipball/%s", repoOwner, repoName, ref)
	tmpFileName := fmt.Sprintf(".%d.download", time.Now().UnixNano()%100000000)
	tmpFilePath := SafePath(basePath, tmpFileName)
	destPath := SafePath(basePath, dest)

	defer os.Remove(tmpFilePath) // Clean up temp file on return

	deployCtx["$step"] = "before stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "txAdmin-Deployer-Go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading github zipball: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github zipball download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.Create(tmpFilePath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("saving zipball stream: %w", err)
	}
	tmpFile.Close()

	deployCtx["$step"] = "zip parsed"
	zipReader, err := zip.OpenReader(tmpFilePath)
	if err != nil {
		return fmt.Errorf("opening downloaded zip archive: %w", err)
	}
	defer zipReader.Close()

	if len(zipReader.File) == 0 {
		return errors.New("downloaded zip archive is empty")
	}

	// GitHub zipballs always wrap content inside a root folder (e.g. `owner-repo-commit/`)
	firstEntry := filepath.ToSlash(zipReader.File[0].Name)
	rootFolder := strings.Split(firstEntry, "/")[0]

	// Determine prefix path inside the archive to extract
	targetPrefix := path.Join(rootFolder, subpath)
	if !strings.HasSuffix(targetPrefix, "/") {
		targetPrefix += "/"
	}

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	for _, file := range zipReader.File {
		slashedName := filepath.ToSlash(file.Name)

		// Filter files matching target subpath
		if !strings.HasPrefix(slashedName, targetPrefix) {
			continue
		}

		// Calculate relative destination path
		relPath := strings.TrimPrefix(slashedName, targetPrefix)
		if relPath == "" {
			continue
		}

		targetFilePath := SafePath(destPath, relPath)

		if file.FileInfo().IsDir() {
			os.MkdirAll(targetFilePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetFilePath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if copyErr != nil {
			return copyErr
		}
	}

	deployCtx["$step"] = "task finished"
	return nil
}

func taskEnsureDir(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	dest := SafePath(basePath, GetString(options, "path"))
	return os.MkdirAll(dest, 0755)
}

func taskRemovePath(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	target := SafePath(basePath, GetString(options, "path"))
	cleanBase := filepath.Clean(basePath)
	if target == cleanBase {
		return errors.New("cannot remove base folder")
	}
	return os.RemoveAll(target)
}

func taskMovePath(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	src := GetString(options, "src")
	dest := GetString(options, "dest")

	if src == "" || dest == "" {
		return errors.New("invalid options: both src and dest are required")
	}

	srcPath := SafePath(basePath, src)
	destPath := SafePath(basePath, dest)

	// Parse overwrite parameter (supports boolean true or string "true")
	overwrite := options["overwrite"] == true || GetString(options, "overwrite") == "true"

	// Ensure parent directory of target destination exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create target parent directory: %w", err)
	}

	// Handle existing destination path
	if _, err := os.Stat(destPath); err == nil {
		if overwrite {
			if err := os.RemoveAll(destPath); err != nil {
				return fmt.Errorf("failed to remove existing path for overwrite: %w", err)
			}
		} else {
			return fmt.Errorf("destination path '%s' already exists and overwrite is false", dest)
		}
	}

	// Try fast atomic rename first
	if err := os.Rename(srcPath, destPath); err == nil {
		return nil
	}

	// Fallback for cross-device links (if src and dest are on different mount points)
	if err := movePathFallback(srcPath, destPath); err != nil {
		return fmt.Errorf("moving path across filesystems failed: %w", err)
	}

	return nil
}

// movePathFallback recursively copies files and removes source if os.Rename fails
func movePathFallback(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := os.MkdirAll(dest, info.Mode()); err != nil {
			return err
		}

		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			srcChild := filepath.Join(src, entry.Name())
			destChild := filepath.Join(dest, entry.Name())

			if err := movePathFallback(srcChild, destChild); err != nil {
				return err
			}
		}
		return os.Remove(src)
	}

	// File copy
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	in.Close() // Close before removing on Windows
	return os.Remove(src)
}

func taskWriteFile(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	target := SafePath(basePath, GetString(options, "file"))
	data := GetString(options, "data")
	appendMode := options["append"] == true || GetString(options, "append") == "true"

	os.MkdirAll(filepath.Dir(target), 0755)

	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(target, flags, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(data)
	return err
}

func taskReplaceString(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	// Handle file string or slice of strings
	var files []string
	switch v := options["file"].(type) {
	case string:
		files = append(files, v)
	case []interface{}:
		for _, f := range v {
			files = append(files, f.(string))
		}
	}

	search := GetString(options, "search")
	replace := GetString(options, "replace")
	mode := GetString(options, "mode")
	if mode == "" {
		mode = "template"
	}

	for _, f := range files {
		target := SafePath(basePath, f)
		b, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		content := string(b)

		switch mode {
		case "template":
			replacedVar := ReplaceVars(replace, deployCtx)
			content = strings.ReplaceAll(content, search, replacedVar)
		case "all_vars":
			content = ReplaceVars(content, deployCtx)
		case "literal":
			content = strings.ReplaceAll(content, search, replace)
		}

		err = os.WriteFile(target, []byte(content), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func taskConnectDatabase(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	host := GetString(deployCtx, "dbHost")
	port := GetInteger(deployCtx, "dbPort", 3306)
	user := GetString(deployCtx, "dbUsername")
	pass := GetString(deployCtx, "dbPassword")
	dbName := GetString(deployCtx, "dbName")
	dbDelete := GetBool(deployCtx, "dbDelete")

	// Connect without DB name to create it first
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?multiStatements=true&allowNativePasswords=true", user, pass, host, port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	if dbDelete {
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
		if err != nil {
			return err
		}
	}
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8 COLLATE utf8_general_ci", dbName))
	if err != nil {
		return err
	}

	// Reconnect to specific DB
	dsnDb := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?multiStatements=true&allowNativePasswords=true", user, pass, host, port, dbName)
	dbConnection, err := sql.Open("mysql", dsnDb)
	if err != nil {
		return err
	}

	var tableCount int
	query := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?"
	if err := dbConnection.QueryRowContext(ctx, query, dbName).Scan(&tableCount); err != nil {
		dbConnection.Close()
		return fmt.Errorf("failed to check existing tables: %w", err)
	}

	if tableCount > 0 {
		dbConnection.Close()
		return fmt.Errorf("database '%s' is not empty (%d table(s) found); please clear the database, choose a clean database name or re-run with the -dbdelete flag (provide yes)", dbName, tableCount)
	}

	deployCtx["dbConnection"] = dbConnection
	return nil
}

func taskQueryDatabase(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	dbVal, ok := deployCtx["dbConnection"]
	if !ok || dbVal == nil {
		return errors.New("database connection not found. Run connect_database before query_database")
	}
	db := dbVal.(*sql.DB)

	var query string
	if file := GetString(options, "file"); file != "" {
		target := SafePath(basePath, file)
		b, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		query = string(b)
	} else {
		query = GetString(options, "query")
	}

	_, err := db.Exec(query)
	return err
}

func taskUnzip(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	src := SafePath(basePath, GetString(options, "src"))
	dest := SafePath(basePath, GetString(options, "dest"))

	os.MkdirAll(dest, 0755)

	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func taskLoadVars(ctx context.Context, options RecipeTask, basePath string, deployCtx DeployerCtx) error {
	src := SafePath(basePath, GetString(options, "src"))
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	var inData map[string]any
	if err := json.Unmarshal(b, &inData); err != nil {
		return err
	}
	for k, v := range inData {
		deployCtx[k] = v
	}
	delete(deployCtx, "dbConnection") // Safe guard
	return nil
}
