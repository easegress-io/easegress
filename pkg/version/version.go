package version

import "fmt"

var (
	// RELEASE returns the release version
	RELEASE = "UNKNOWN"
	// REPO returns the git repository URL
	REPO = "UNKNOWN"
	// COMMIT returns the short sha from git
	COMMIT = "UNKNOWN"

	API   = "v2"
	Short = fmt.Sprintf("EaseGateway %s", RELEASE)
	Long  = fmt.Sprintf("EaseGateway release: %s, repo: %s, commit: %s", RELEASE, REPO, COMMIT)
)
