.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build local binaries
	bash ./scripts/compile_commands.sh

test: ## Run golang tests
	go test -race ./...

coverage-output: ## Get coverage output to cover.out
	go test ./... -coverprofile=cover.out

coverage-show-func: ## Display coverage output
	go tool cover -func cover.out
