VERSION=0.0.4
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION}"
GO111MODULE=on

all: mackerel-plugin-httping

.PHONY: mackerel-plugin-httping linux check lint

mackerel-plugin-httping: main.go
	go build $(LDFLAGS) -o mackerel-plugin-httping

linux: main.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o mackerel-plugin-httping

check:
	go test -v ./...
	go test -race ./...

lint:
	golangci-lint run --timeout 5m ./...