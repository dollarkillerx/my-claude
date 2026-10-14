VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/dollarkillerx/my-claude/internal/cli.Version=$(VERSION)

.PHONY: build install test e2e fmt clean dist

build:
	go build -ldflags '$(LDFLAGS)' -o bin/cswitch ./cmd/cswitch

install:
	go install -ldflags '$(LDFLAGS)' ./cmd/cswitch

test:
	go vet ./...
	go vet -tags e2e ./...
	go test ./...

# Uses the real macOS keychain with test-only items holding dummy values.
e2e:
	go test -tags e2e -count=1 ./internal/cli

fmt:
	gofmt -w .

# Release archives for macOS arm64 and Linux x86-64, plus checksums.
dist:
	rm -rf dist && mkdir -p dist
	for target in darwin/arm64 linux/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; name=cswitch_$(VERSION)_$${os}_$${arch}; \
		mkdir -p dist/$$name && \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$$name/cswitch ./cmd/cswitch && \
		cp LICENSE README.md README.zh-CN.md README.ja.md dist/$$name/ && \
		tar -C dist -czf dist/$$name.tar.gz $$name && rm -rf dist/$$name || exit 1; \
	done
	cd dist && shasum -a 256 *.tar.gz > checksums.txt

clean:
	rm -rf bin dist
