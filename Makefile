PREFIX ?= $(HOME)/.local

.PHONY: build test install
build:
	go build -o pingbar .

test:
	go test ./...

install: build
	mkdir -p $(PREFIX)/bin
	cp pingbar $(PREFIX)/bin/pingbar
	@echo "Installed $(PREFIX)/bin/pingbar. Run 'pingbar install' to add the menu-bar item."
