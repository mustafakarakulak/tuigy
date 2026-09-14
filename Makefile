# tuigy — everything here is a convenience. Nothing in the project needs make:
# `go build ./...` and `go test ./...` work on their own.
#
# Run `make` on its own to see what is available.

BINARY      := tuigy
COVER_FILE  := coverage.out
COVER_FLOOR := 90
# The oldest Go and git tuigy supports, which is what the Linux check runs.
LINUX_IMAGE := golang:1.24

.DEFAULT_GOAL := help

.PHONY: help
help: ## list the targets
	@grep -hE '^[a-z-]+:.*##' $(MAKEFILE_LIST) \
		| sed 's/:.*## /\t/' \
		| awk -F'\t' '{printf "  \033[1m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## build the binary into ./tuigy
	go build -o $(BINARY) ./cmd/tuigy

.PHONY: install
install: ## install the binary into GOPATH/bin
	go install ./cmd/tuigy

.PHONY: run
run: build ## build, then run it here
	./$(BINARY)

.PHONY: test
test: ## run the test suite
	go test ./...

.PHONY: race
race: ## run the test suite under the race detector
	go test -race ./...

.PHONY: cover
cover: ## measure coverage across packages and enforce the floor
	go test -coverpkg=./... -coverprofile=$(COVER_FILE) ./...
	@go tool cover -func=$(COVER_FILE) | tail -1
	@total=$$(go tool cover -func=$(COVER_FILE) | tail -1 | awk '{print $$3}' | tr -d '%'); \
	awk -v total="$$total" -v floor="$(COVER_FLOOR)" \
		'BEGIN { exit (total + 0 >= floor) ? 0 : 1 }' \
		|| { echo "coverage $$total% is below the $(COVER_FLOOR)% floor"; exit 1; }

.PHONY: cover-html
cover-html: cover ## open the coverage report in a browser
	go tool cover -html=$(COVER_FILE)

.PHONY: fmt
fmt: ## format the source
	gofmt -w .

.PHONY: vet
vet: ## run go vet
	go vet ./...

.PHONY: check
check: ## what to run before pushing: formatting, vet, tests
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "not gofmt'd:"; echo "$$unformatted"; exit 1; fi
	go vet ./...
	go test ./...

.PHONY: linux
linux: ## run the suite on Linux, where CI runs it (needs Docker)
	@docker version >/dev/null 2>&1 || { echo "Docker is not running"; exit 1; }
	docker run --rm -v "$(CURDIR)":/src:ro -w /src $(LINUX_IMAGE) sh -c '\
		git config --global user.email ci@example.com; \
		git config --global user.name "tuigy CI"; \
		go vet ./... && go test -race ./...'

.PHONY: snapshot
snapshot: ## build the release artefacts without publishing (needs goreleaser)
	goreleaser release --snapshot --clean --skip=publish

.PHONY: clean
clean: ## remove build and coverage output
	rm -rf $(BINARY) $(COVER_FILE) dist
