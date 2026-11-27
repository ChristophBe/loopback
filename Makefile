.DEFAULT_GOAL := help
COMPOSE := docker compose -f deploy/compose.yaml --env-file .env

.PHONY: help gen check-gen lint lint-go lint-web lint-plugin lint-docs lint-specs lint-content \
	test test-go test-web test-plugin test-content up down reset import

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

gen: ## Generate code from the specs
	cd backend && go generate ./...
	cd web && pnpm install --frozen-lockfile && pnpm run gen
	docker run --rm --user "$$(id -u):$$(id -g)" -v "$(CURDIR):/w" -w /w openapitools/openapi-generator-cli:v7.12.0 generate \
		-i api/openapi.yaml -g kotlin \
		--global-property models,modelTests=false,modelDocs=false,apiTests=false,apiDocs=false,supportingFiles=false \
		-p library=jvm-okhttp4,serializationLibrary=gson,packageName=dev.loopback.api,modelPackage=dev.loopback.api.model \
		-o plugin/gen-tmp
	rm -rf plugin/src/gen && mkdir -p plugin/src/gen && cp -R plugin/gen-tmp/src/main/kotlin/. plugin/src/gen/ && rm -rf plugin/gen-tmp

check-gen: gen ## Fail when generated code is out of date
	git diff --exit-code -- backend plugin/src/gen web/src/gen
	@test -z "$$(git ls-files --others --exclude-standard -- backend plugin/src/gen web/src/gen)" || \
		(echo "untracked generated files"; git ls-files --others --exclude-standard -- backend plugin/src/gen web/src/gen; exit 1)

lint: lint-go lint-web lint-plugin lint-docs lint-specs lint-content ## Run every linter

lint-go: ## Lint the backend
	cd backend && golangci-lint run --config ../.golangci.yml ./...

lint-web: ## Lint the web app
	cd web && pnpm install --frozen-lockfile && pnpm run lint && pnpm run format:check && pnpm run typecheck

lint-plugin: ## Lint the plugin
	cd plugin && ./gradlew ktlintCheck detekt

lint-docs: ## Lint the Markdown files
	cd tools && pnpm install --frozen-lockfile && pnpm run lint:docs

lint-specs: ## Lint the OpenAPI spec
	cd tools && pnpm install --frozen-lockfile && pnpm run lint:specs

lint-content: ## Validate lessons, quizzes, flashcards, reviews and fixtures against the schemas
	cd tools && pnpm install --frozen-lockfile && pnpm run lint:content

test: test-go test-web test-plugin test-content ## Run every test

test-go: ## Test the backend (set TEST_DATABASE_URL for the ent tests)
	cd backend && go test -race -cover ./...

test-web: ## Test the web app
	cd web && pnpm install --frozen-lockfile && pnpm test

test-plugin: ## Test the plugin
	cd plugin && ./gradlew test

test-content: ## Test the content validator
	cd tools && pnpm install --frozen-lockfile && pnpm test

.env:
	cp deploy/.env.example .env

up: .env ## Start the local stack
	$(COMPOSE) up --build -d --wait
	@echo "web http://localhost:5173  api http://localhost:8080/api/v1  mail http://localhost:8025"

down: ## Stop the local stack
	$(COMPOSE) down

reset: .env ## Delete the database and start again
	$(COMPOSE) down -v
	$(MAKE) up

import: .env ## Re-run the content import
	CONTENT_SHA="$$(git rev-parse HEAD)" $(COMPOSE) run --rm import
