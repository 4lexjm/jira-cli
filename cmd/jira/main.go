package main

import (
	"os"

	"github.com/atlassian/jira-cli/internal/build"
	"github.com/atlassian/jira-cli/internal/jiracmd"
)

func main() {
	os.Exit(jiracmd.Main(build.Version))
}
