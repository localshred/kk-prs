# kk-prs dev Makefile. `make install.dev` builds this plugin into the
# managed plugin dir where kk discovers it.

XDG_DATA_HOME ?= $(HOME)/.local/share
MANAGED_BIN := $(XDG_DATA_HOME)/koolkit/bin
BINARY := kk-prs

.PHONY: install.dev

install.dev:
	@mkdir -p "$(MANAGED_BIN)"
	go build -o "$(MANAGED_BIN)/$(BINARY)" .
	@echo "installed $(BINARY) -> $(MANAGED_BIN)/$(BINARY)"
