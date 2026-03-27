PREFIX ?= $(HOME)/.local

build:
	go build -o git-better-branch .

install: build
	install -d $(PREFIX)/bin
	install -m 755 git-better-branch $(PREFIX)/bin/

uninstall:
	rm -f $(PREFIX)/bin/git-better-branch

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f git-better-branch

.PHONY: build install uninstall test lint clean
