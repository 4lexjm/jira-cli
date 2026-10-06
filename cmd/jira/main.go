package main

import (
	"os"

	"github.com/atlassian/jira-cli/internal/jiracmd"
)

var version = "dev"

func main() {
	os.Exit(jiracmd.Main(version))
}
