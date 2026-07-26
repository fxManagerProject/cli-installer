package recipe

// Step represents the current stage of the deployer
type Step string

const (
	StepReview    Step = "review"
	StepInput     Step = "input"
	StepRun       Step = "run"
	StepConfigure Step = "configure"
)

// RecipeTask represents a single task in the recipe YAML
type RecipeTask map[string]any

// YamlRecipe represents the structure of the incoming YAML file
type YamlRecipe struct {
	Engine        *int           `yaml:"$engine,omitempty"`
	MinFxVersion  *int           `yaml:"$minFxVersion,omitempty"`
	Onesync       *string        `yaml:"$onesync,omitempty"`
	SteamRequired *bool          `yaml:"$steamRequired,omitempty"`
	Name          string         `yaml:"name"`
	Author        string         `yaml:"author"`
	Description   string         `yaml:"description"`
	Variables     map[string]any `yaml:"variables"`
	Tasks         []RecipeTask   `yaml:"tasks"`
}

// ParsedRecipe is the validated and parsed output
type ParsedRecipe struct {
	Raw                 string
	Name                string
	Author              string
	Description         string
	Variables           map[string]any
	Tasks               []RecipeTask
	Onesync             string
	FxserverMinVersion  int
	RecipeEngineVersion int
	SteamRequired       bool
	RequireDBConfig     bool
}

// DeployerCtx holds execution context, including database connections and variables
type DeployerCtx map[string]any
