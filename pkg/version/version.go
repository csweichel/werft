package version

import "fmt"

var (
	// Version is the semver release name of this build
	Version string
	// Commit is the commit hash this build was created from
	Commit string
	// Date is the time when this build was created
	Date string
)

// Print writes the version info to stdout
func Print() {
	fmt.Printf("Version:    %s\n", Version)
	fmt.Printf("Commit:     %s\n", Commit)
	fmt.Printf("Build date: %s\n", Date)
}
