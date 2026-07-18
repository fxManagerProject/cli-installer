package recipe

import (
	"errors"
	"fmt"
)

var ErrNotImplemented = errors.New("txAdmin compat recipe module is currently unavailable")

// Fetch is a placeholder function for downloading and extracting a recipe into destDir.
// In the future, this will link a GitHub repository with a txAdmin installation recipe,
// download the contents, and execute steps defined in recipe.yaml (e.g., downloading
// releases, moving files, running SQL, replacing placeholders).
//
// Accepts a bare GitHub repo URL (https://github.com/owner/repo)
func Fetch(rawURL, destDir string) error {
	fmt.Println("[WARN] The txAdmin recipe installation module is not yet implemented.")
	fmt.Printf("[WARN] Cannot process recipe for: %s\n", rawURL)

	return ErrNotImplemented
}

// ToDo: implement txAdmins deployer module to provide a solution for users to install the server
// https://github.com/citizenfx/txAdmin/tree/8a9a41410000fd92a527fe11bae2b8eeeb8b10e0/core/deployer
