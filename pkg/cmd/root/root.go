package root

import (
	"github.com/spf13/cobra"

	"github.com/atlassian/jira-cli/pkg/cmd/auth"
	"github.com/atlassian/jira-cli/pkg/cmdutil"
)

// NewCmdRoot builds the root cobra command for the jira CLI.
func NewCmdRoot(f *cmdutil.Factory, version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jira",
		Short: "Jira CLI for AI agents and humans",
		Long: `jira is a unified CLI for Jira Data Center and Jira Cloud.

It is designed for AI agents (MCP server, structured JSON output) and humans alike.

Authentication:
  Kerberos (DC+AD):   jira auth login https://jira.example.com --auth-method kerberos
  Browser SSO (DC):   jira auth login https://jira.example.com --browser
  PAT bearer (DC):    jira auth login https://jira.example.com --token <PAT>
  API token (Cloud):  jira auth login https://myorg.atlassian.net --kind cloud \
                        --email me@example.com --token <TOKEN>

Environment variables (headless/CI):
  JIRA_HOST, JIRA_TOKEN, JIRA_AUTH_METHOD, JIRA_SESSION_COOKIE
  JIRA_OAUTH_CLIENT_ID, JIRA_OAUTH_CLIENT_SECRET (OAuth 2.0 M2M)`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Global flags (inherited by all subcommands).
	cmd.PersistentFlags().StringP("context", "c", "", "Active Jira context name")
	cmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	cmd.PersistentFlags().Bool("yaml", false, "Output in YAML format")
	cmd.PersistentFlags().String("jq", "", "Apply a jq expression to JSON output")
	cmd.PersistentFlags().Bool("debug", false, "Show HTTP request/response details (sets JIRA_HTTP_DEBUG=1)")

	// Subcommands.
	cmd.AddCommand(auth.NewCmdAuth(f))
	// Additional subcommands (issue, project, sprint, …) registered here.

	return cmd
}
