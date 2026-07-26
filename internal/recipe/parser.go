package recipe

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const RECIPE_DEPLOYER_VERSION = 3

// ParseRecipe validates a Recipe file from a raw YAML string
func ParseRecipe(rawRecipe string, fxsVersion int) (*ParsedRecipe, error) {
	var recipe YamlRecipe
	err := yaml.Unmarshal([]byte(rawRecipe), &recipe)
	if err != nil {
		return nil, fmt.Errorf("invalid yaml: %w", err)
	}

	if len(recipe.Tasks) == 0 {
		return nil, errors.New("no tasks array found")
	}

	out := &ParsedRecipe{
		Raw:             strings.TrimSpace(rawRecipe),
		Name:            strings.TrimSpace(recipe.Name),
		Author:          strings.TrimSpace(recipe.Author),
		Description:     strings.TrimSpace(recipe.Description),
		Variables:       make(map[string]any),
		Tasks:           []RecipeTask{},
		RequireDBConfig: false,
	}

	if out.Name == "" {
		out.Name = "unnamed"
	}
	if out.Author == "" {
		out.Author = "unknown"
	}

	// Meta tag checks
	if recipe.Onesync != nil {
		osVal := strings.TrimSpace(*recipe.Onesync)
		if osVal != "off" && osVal != "legacy" && osVal != "on" {
			return nil, fmt.Errorf(`the onesync option "%s" is not supported`, osVal)
		}
		out.Onesync = osVal
	}

	if recipe.MinFxVersion != nil {
		if *recipe.MinFxVersion > fxsVersion {
			return nil, fmt.Errorf("this recipe requires FXServer v%d or above", *recipe.MinFxVersion)
		}
		out.FxserverMinVersion = *recipe.MinFxVersion
	}

	if recipe.Engine != nil {
		if *recipe.Engine < RECIPE_DEPLOYER_VERSION {
			return nil, fmt.Errorf("unsupported '$engine' version %d", *recipe.Engine)
		}
		out.RecipeEngineVersion = *recipe.Engine
	}

	if recipe.SteamRequired != nil && *recipe.SteamRequired {
		out.SteamRequired = true
	}

	// Validate tasks
	for i, task := range recipe.Tasks {
		action := GetString(task, "action")
		if action == "" {
			return nil, fmt.Errorf("[task%d] no action specified", i+1)
		}
		engineTask, exists := Engine[action]
		if !exists {
			return nil, fmt.Errorf("[task%d] unknown action '%s'", i+1, action)
		}
		if !engineTask.Validate(task) {
			return nil, fmt.Errorf("[task%d:%s] invalid parameters", i+1, action)
		}
		out.Tasks = append(out.Tasks, task)
		if strings.Contains(action, "database") {
			out.RequireDBConfig = true
		}
	}

	// Validate variables
	protectedVars := map[string]bool{
		"svLicense": true, "dbHost": true, "dbUsername": true,
		"dbPassword": true, "dbName": true, "dbConnection": true, "dbPort": true,
	}
	if recipe.Variables != nil {
		for k, v := range recipe.Variables {
			if protectedVars[k] {
				return nil, errors.New("one or more of the variables declared in the recipe are not allowed")
			}
			out.Variables[k] = v
		}
	}

	return out, nil
}
