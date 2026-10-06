BINARY_NAME = jira
BIN_DIR     = ./bin
CMD_DIR     = ./cmd/jira

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     = -X github.com/atlassian/jira-cli/internal/build.Version=$(VERSION)

.PHONY: all build test fmt lint tidy clean check-skills sync-skills

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)


test:
	go test ./...

fmt:
	gofmt -w .
	goimports -w .

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)

# Verify that agent-skill mirrors are up-to-date
check-skills:
	@bash scripts/check-generated-skill.sh

check-generated-skill:
	@bash scripts/check-generated-skill.sh

# Regenerate .claude/skills/jira and .agents/skills/jira from skills/jira
sync-skills:
	@bash scripts/sync-skills.sh

