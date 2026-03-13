.PHONY: fmt test build verify

fmt:
	gofmt -w $$(find . -name '*.go' -print)

test:
	go test ./...

build:
	go build ./cmd/oascli ./cmd/oasclird

verify: fmt test build
