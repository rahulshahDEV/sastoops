BINARY := sastoops
VERSION := $(shell grep -m1 'const Version' internal/config/config.go | sed 's/.*"\(.*\)"/\1/')

.PHONY: build test lint clean release

build:
	go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) .

test:
	go test ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l .)"

clean:
	rm -rf bin dist

release:
	mkdir -p dist
	for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/$(BINARY)-$$os-$$arch ./ ; \
		done; \
	done
	ls -lh dist/