-include .env
export

CLIENT_BIN   := callsign-client
SERVER_BIN   := fake-server
PLIST_LABEL  := com.callsign.client
PLIST_PATH   := $(HOME)/Library/LaunchAgents/$(PLIST_LABEL).plist
PROJECT_DIR  := $(shell pwd)

.PHONY: all build build-client build-server run-client run-server \
        test test-e2e vet fmt tidy agent-install agent-uninstall agent-status agent-logs clean help \
        firmware-build firmware-upload firmware-monitor firmware-check firmware-test \
        firmware-clean firmware-size

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

firmware-check: ## Static analysis of the firmware (cppcheck via PlatformIO)
	cd firmware && pio check --fail-on-defect=high

firmware-test: ## Run firmware unit tests on the host (no ESP32 needed)
	cd firmware && pio test -e native

firmware-clean: ## Remove all firmware build artifacts (.pio, generated config)
	cd firmware && pio run -t clean && rm -rf .pio callsign/wifi_config.h

firmware-size: firmware-build ## Show firmware size vs board limits (fails if over)
	@cd firmware && pio run | sed -n -E \
		-e 's/^(RAM|Flash):.*used ([0-9]+) bytes from ([0-9]+) bytes.*/\1 \2 \3/p' | \
	while read region used total; do \
		pct=$$((used * 100 / total)); \
		if [ $$used -gt $$total ]; then \
			printf "%-6s %8d / %d bytes (%d%%)  *** OVER LIMIT ***\n" $$region $$used $$total $$pct; \
			exit 1; \
		else \
			printf "%-6s %8d / %d bytes (%d%%)\n" $$region $$used $$total $$pct; \
		fi; \
	done

## launchd (run the client always in background, restart on failure)

agent-install: build-client ## Install & start the client as a launchd agent
	@mkdir -p $(HOME)/Library/LaunchAgents
	@sed -e 's|__PROJECT_DIR__|$(PROJECT_DIR)|g' \
		client/launchd/$(PLIST_LABEL).plist > $(PLIST_PATH)
	@launchctl unload $(PLIST_PATH) 2>/dev/null || true
	launchctl load $(PLIST_PATH)
	@echo "Installed. Logs: /tmp/callsign.log — status: make agent-status"

agent-uninstall: ## Stop & remove the launchd agent (and its log file)
	@launchctl unload $(PLIST_PATH) 2>/dev/null || true
	@rm -f $(PLIST_PATH) /tmp/callsign.log
	@echo "Agent removed."

agent-status: ## Show whether the agent is running
	@launchctl list | grep $(PLIST_LABEL) || echo "not running"

agent-logs: ## Tail the launchd agent logs (Ctrl-C to stop)
	tail -f /tmp/callsign.log

## Misc

clean: ## Remove built binaries
	rm -rf bin

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
