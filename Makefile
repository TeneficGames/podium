# podium
# https://github.com/TeneficGames/podium
# Licensed under the MIT license:
# http://www.opensource.org/licenses/mit-license
# Copyright © 2026 Tenefic Games
# Forked from
# https://github.com/topfreegames/podium
# Copyright © 2016 Top Free Games

GODIRS = $(shell go list ./... | grep -v /vendor/ | sed s@github.com/TeneficGames/podium@.@g | egrep -v "^[.]$$")
MYIP = $(shell ifconfig | egrep inet | egrep -v inet6 | egrep -v 127.0.0.1 | awk ' { print $$2 } ')
OS = "$(shell uname | awk '{ print tolower($$0) }')"
PROTOTOOL := go run github.com/uber/prototool/cmd/prototool
LOCAL_GO_MODCACHE = $(shell go env GOMODCACHE)
BUF := go run github.com/bufbuild/buf/cmd/buf@v1.72.0
MOCKGENERATE := go run go.uber.org/mock/mockgen@v0.6.0
GOTESTSUM := go run gotest.tools/gotestsum@v1.13.0
BENCH_COUNT ?= 5
BENCH_REDIS_IMAGE ?= redis:8.2-alpine
BENCH_REDIS_CONTAINER ?= podium-benchmark-redis
BENCH_PID_FILE ?= /tmp/podium-benchmark.pid

help: Makefile ## Show list of commands
	@echo "Choose a command run in "$(PROJECT_NAME)":"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-40s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

.PHONY: bench-podium-app bench-podium-app-kill bench-podium-app-run bench-redis bench-redis-kill bench-run bench-seed bench-tiebreak build compose-down compose-test compose-up-api compose-up-dependencies coverage-check docker-build docker-run docker-run-basic-auth docker-run-redis help lint mock-generate mock-lib proto proto-check proto-setup proto-tools redis-cluster-test rtfd setup setup-docs setup-hooks clear-hooks test test-client test-helm test-helm-minikube test-leaderboard test-podium test-redis-cluster test-redis-compat test-unit

setup-hooks: ## Create pre-commit git hooks
	@cd .git/hooks && ln -sf ../../hooks/pre-commit.sh pre-commit

clear-hooks: ## Remove pre-commit git hooks
	@cd .git/hooks && rm pre-commit

setup: ## Download dependencies for all Go modules
	@go mod download
	@cd leaderboard && go mod download
	@cd proto && go mod download
	@cd client && go mod download

setup-docs: ## Install dependencies necessary for building docs
	@pip2.7 install -q --log /tmp/pip.log --no-cache-dir sphinx recommonmark sphinx_rtd_theme

build: ## Build the project
	@go build -o ./bin/podium ./main.go

run: ## Execute the project
	@go run main.go start

test: test-podium test-leaderboard test-client ## Execute all tests

test-unit: ## Execute Redis-independent unit tests
	@go test ./cmd ./observability
	@cd leaderboard && go test ./database ./enriching ./expiration ./service
	@cd proto && go test ./...
	@cd client && go test ./...

test-helm: ## Lint, render, package, and unit test the Helm chart
	@./scripts/test-helm.sh

test-helm-minikube: ## Build and smoke test the Helm chart in Minikube
	@./scripts/test-helm-minikube.sh

test-podium: ## Execute all API tests
	@mkdir -p _build/test-results
	@$(GOTESTSUM) --junitfile=_build/test-results/podium.xml -- -coverprofile=podium.coverprofile ./...

test-leaderboard: ## Execute all leaderboard tests
	@mkdir -p _build/test-results
	@cd leaderboard && $(GOTESTSUM) --junitfile=../_build/test-results/leaderboard.xml -- -coverprofile=leaderboard.coverprofile ./...

test-redis-compat: ## Test leaderboard behavior against a real Redis server
	@test -n "$$PODIUM_TEST_REDIS_VERSION" || (echo "PODIUM_TEST_REDIS_VERSION is required" && exit 1)
	@cd leaderboard && PODIUM_TEST_REDIS_EXTERNAL=true go test -tags=rediscompat ./...

test-redis-cluster: ## Test leaderboard behavior against a real three-primary Redis Cluster
	@test -n "$$PODIUM_REDIS_ADDRS" || (echo "PODIUM_REDIS_ADDRS is required" && exit 1)
	@cd leaderboard && PODIUM_TEST_REDIS_EXTERNAL=true PODIUM_REDIS_CLUSTER_ENABLED=true go test -tags=rediscluster ./...

test-client: ## Execute all client tests
	@mkdir -p _build/test-results
	@cd client && $(GOTESTSUM) --junitfile=../_build/test-results/client.xml -- -coverprofile=client.coverprofile ./...

lint: ## Run golangci-lint for all Go modules
	@golangci-lint run ./...
	@cd leaderboard && golangci-lint run ./...
	@cd proto && golangci-lint run ./...
	@cd client && golangci-lint run ./...

coverage: ## Generate code coverage file
	@mkdir -p _build
	@rm -f _build/test-coverage-all.out
	@echo "mode: count" > _build/test-coverage-all.out
	@bash -eu -o pipefail -c 'for f in podium.coverprofile leaderboard/leaderboard.coverprofile client/client.coverprofile; do tail -n +2 $$f >> _build/test-coverage-all.out; done'

coverage-check: coverage ## Require at least 80% coverage in every production Go package
	@awk -v threshold=80 -f scripts/check-package-coverage.awk _build/test-coverage-all.out

test-coverage-html: test coverage ## Generate HTML coverage reports for all Go modules
	@go tool cover -html=podium.coverprofile -o _build/podium-coverage.html
	@cd leaderboard && go tool cover -html=leaderboard.coverprofile -o ../_build/leaderboard-coverage.html
	@cd client && go tool cover -html=client.coverprofile -o ../_build/client-coverage.html

docker-build: ## Build docker-compose services
	@docker build -f ./build/Dockerfile -t podium .

docker-run: ## Run podium inside Docker
	@docker run -i -t --rm -e PODIUM_REDIS_HOST=$(MYIP) -e PODIUM_REDIS_PORT=6379 -p 8880:8880 podium

docker-run-redis: ## Run a redis instance in Docker
	@docker run --name=redis -d -p 6379:6379 redis:8.2-alpine

docker-run-basic-auth: ## Run podium inside Docker and setup basic auth (admin:12345)
	@docker run -i -t --rm -e PODIUM_BASICAUTH_USERNAME=admin -e PODIUM_BASICAUTH_PASSWORD=12345 -e PODIUM_REDIS_HOST=$(MYIP) -e PODIUM_REDIS_PORT=6379 -p 8080:80 podium

deployments/docker-compose.yaml: deployments/docker-compose-model.yaml
	@sed "s%<<LOCAL_GO_MODCACHE>>%${LOCAL_GO_MODCACHE}%g" $< > $@

compose-up-dependencies: deployments/docker-compose.yaml ## Run all dependencies using docker-compose
	@docker compose -f $< up -d --wait redis-node-0 redis-node-1 redis-node-2 redis-standalone
	@docker compose -f $< run --rm initialize-cluster

compose-up-api: deployments/docker-compose.yaml ## Initialize api on composer environment
	@docker compose -f $< up -d --build podium-api

compose-test: deployments/docker-compose.yaml compose-up-dependencies ## Execute podium tests using docker-compose
	@docker compose -f $< run --rm --no-deps podium-test

compose-down: deployments/docker-compose.yaml ## Stop all dependency containers
	@docker compose -f $< down

bench-podium-app: build bench-podium-app-run ## Execute benchmark app

bench-podium-app-run: bench-podium-app-kill ## Execute benchmark app
	@rm -f /tmp/podium-bench.log
	@nohup ./bin/podium start -p 8888 -g 8889 -q -c ./config/perf.yaml > /tmp/podium-bench.log 2>&1 & echo $$! > $(BENCH_PID_FILE)
	@for i in $$(seq 1 50); do curl --fail --silent http://localhost:8888/healthcheck >/dev/null && exit 0; sleep 0.1; done; cat /tmp/podium-bench.log; exit 1
	@echo "Podium started at http://localhost:8888. GRPC at 8889."

bench-podium-app-kill: ## Stop benchmark app
	@if test -f $(BENCH_PID_FILE); then kill "$$(cat $(BENCH_PID_FILE))" 2>/dev/null || true; rm -f $(BENCH_PID_FILE); fi

bench-redis: bench-redis-kill ## Start the Redis version used by local benchmarks
	@docker run --rm --detach --name $(BENCH_REDIS_CONTAINER) --publish 6379:6379 $(BENCH_REDIS_IMAGE)

bench-redis-kill: ## Stop the local benchmark Redis container
	@docker rm --force $(BENCH_REDIS_CONTAINER) >/dev/null 2>&1 || true

bench-run: ## Run HTTP benchmarks five times by default
	@go test ./bench -run '^$$' -bench . -benchmem -count $(BENCH_COUNT)

bench-tiebreak: ## Compare deterministic tie-break implementations
	@cd leaderboard && go test ./database/redis -run '^$$' -bench '^BenchmarkTieBreakStrategies$$' -benchmem -count $(BENCH_COUNT)

bench-seed: ## Seed Redis for large-volume manual benchmarks
	@cd bench/seed && go run .

rtfd: ## Build and open podium documentation
	@rm -rf docs/_build
	@sphinx-build -b html -d ./docs/_build/doctrees ./docs/ docs/_build/html
	@open docs/_build/html/index.html

mock-lib: ## Generate mocks
	@cd client && $(MOCKGENERATE) -destination=mocks/podium.go -package=mocks github.com/TeneficGames/podium/client Client

mock-generate:
	$(MOCKGENERATE) -source=leaderboard/enriching/interfaces.go -destination=leaderboard/mocks/enriching.go
	$(MOCKGENERATE) -source=leaderboard/database/expiration.go -destination=leaderboard/database/expiration_mock.go -package=database

proto-setup: proto-tools
	@$(BUF) dep update

proto-tools:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0

proto-check: proto-tools ## Lint and build protobuf schemas
	@mkdir -p _build
	@$(BUF) format -d --exit-code
	@$(BUF) lint
	@$(BUF) build -o _build/proto.binpb

proto: proto-tools ## Generate protobuf files
	@rm proto/podium/api/v1/*.go > /dev/null 2>&1 || true
	@$(BUF) generate
