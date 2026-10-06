// Package jiracmd wires the root Cobra command and factory together.
package jiracmd

import (
	"os"

	"github.com/atlassian/jira-cli/internal/config"
	"github.com/atlassian/jira-cli/pkg/cmd/root"
	"github.com/atlassian/jira-cli/pkg/cmdutil"
	"github.com/atlassian/jira-cli/pkg/iostreams"
)

// Main is the CLI entry point called from cmd/jira/main.go.
func Main(version string) int {
	ios := iostreams.System()

	f := &cmdutil.Factory{
		AppVersion:     version,
		ExecutableName: "jira",
		IOStreams:       ios,
		Config:          config.Load,
	}

	// Honour JIRA_HTTP_DEBUG globally.
	if os.Getenv("JIRA_HTTP_DEBUG") == "1" {
		os.Setenv("JIRA_HTTP_DEBUG", "1")
	}

	rootCmd := root.NewCmdRoot(f, version)

	if err := rootCmd.Execute(); err != nil {
		return 1
	}
	return 0
}
