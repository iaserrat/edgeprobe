BINARY := edgeprobe
BUILD_DIR := bin
PREFIX ?= /usr/local
BINDIR := $(PREFIX)/bin
CONFIG_DIR ?= /etc/edgeprobe
LOG_DIR ?= /var/log/edgeprobe
SYSTEMD_DIR ?= /etc/systemd/system
GOOS ?=
GOARCH ?=
GOARM ?=

.PHONY: all build build-pi build-pi64 clean install install-config install-service enable disable status logs uninstall uninstall-purge

all: build

build:
	mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) go build -o $(BUILD_DIR)/$(BINARY) ./cmd/edgeprobe

# Raspberry Pi (32-bit). Override GOARM if needed: `make build-pi GOARM=6`
build-pi:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=$(or $(GOARM),7) go build -o $(BUILD_DIR)/$(BINARY) ./cmd/edgeprobe

# Raspberry Pi (64-bit)
build-pi64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY) ./cmd/edgeprobe

clean:
	rm -rf $(BUILD_DIR)

install: build install-config
	install -d $(BINDIR)
	install -m 755 $(BUILD_DIR)/$(BINARY) $(BINDIR)/$(BINARY)
	install -d $(LOG_DIR)

install-config:
	install -d $(CONFIG_DIR)
	[ -f $(CONFIG_DIR)/config.toml ] || install -m 644 config.example.toml $(CONFIG_DIR)/config.toml

install-service:
	install -m 644 scripts/edgeprobe.service $(SYSTEMD_DIR)/edgeprobe.service
	systemctl daemon-reload

enable: install-service
	systemctl enable --now edgeprobe

disable:
	systemctl disable --now edgeprobe || true

status:
	systemctl status edgeprobe

logs:
	journalctl -u edgeprobe -f

uninstall: disable
	rm -f $(BINDIR)/$(BINARY)
	rm -f $(SYSTEMD_DIR)/edgeprobe.service
	systemctl daemon-reload

uninstall-purge: uninstall
	rm -rf $(CONFIG_DIR)
	rm -rf $(LOG_DIR)
