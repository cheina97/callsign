-include .env
export

CLIENT_BIN   := callsign-client
SERVER_BIN   := fake-server
PLIST_LABEL  := com.callsign.client
PLIST_PATH   := $(HOME)/Library/LaunchAgents/$(PLIST_LABEL).plist
PROJECT_DIR  := $(shell pwd)

.PHONY: all build build-client build-server run-client run-server \
        test test-e2e vet fmt tidy install-agent uninstall-agent agent-status clean help \
        firmware-build firmware-upload firmware-monitor

all: build

## Build

build: build-client build-server

build-client: ## Build the macOS client
	cd client && go build -o ../bin/$(CLIENT_BIN) ./cmd/callsign-client

build-server: ## Build the fake server
	cd client && go build -o ../bin/$(SERVER_BIN) ./cmd/fake-server

## Run

run-client: build-client ## Run the client in the foreground (uses .env)
	./bin/$(CLIENT_BIN)

run-server: build-server ## Run the fake server in the foreground
	./bin/$(SERVER_BIN)

## Quality

test: ## Run tests
	cd client && go test ./...

test-e2e: ## Run end-to-end tests (fake server + display client)
	cd client && go test -v ./e2e/

vet: ## go vet
	cd client && go vet ./...

fmt: ## gofmt check (fails if files are unformatted)
	@test -z "$$(gofmt -l client)" || (gofmt -l client && exit 1)

tidy: ## go mod tidy
	cd client && go mod tidy

## Firmware (ESP32, requires PlatformIO: pip install platformio)

firmware-build: ## Compile the ESP32 firmware (also needed before Wokwi simulation)
	cd firmware && pio run

firmware-upload: ## Flash the firmware to a connected ESP32
	cd firmware && pio run -t upload

firmware-monitor: ## Open the serial monitor (115200 baud)
	cd firmware && pio device monitor

## launchd (run the client always in background, restart on failure)

install-agent: build-client ## Install & start the client as a launchd agent
	@mkdir -p $(HOME)/Library/LaunchAgents
	@sed -e 's|__PROJECT_DIR__|$(PROJECT_DIR)|g' \
		launchd/$(PLIST_LABEL).plist > $(PLIST_PATH)
	@launchctl unload $(PLIST_PATH) 2>/dev/null || true
	launchctl load $(PLIST_PATH)
	@echo "Installed. Logs: /tmp/callsign.log — status: make agent-status"

uninstall-agent: ## Stop & remove the launchd agent
	@launchctl unload $(PLIST_PATH) 2>/dev/null || true
	@rm -f $(PLIST_PATH)
	@echo "Agent removed."

agent-status: ## Show whether the agent is running
	@launchctl list | grep $(PLIST_LABEL) || echo "not running"

## Misc

clean: ## Remove built binaries
	rm -rf bin

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
