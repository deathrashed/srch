BINARY := bin/srch
VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test check cross-build clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/srch

test:
	go test ./...

check:
	gofmt -w $$(find cmd internal -name '*.go' -type f)
	go test ./...
	go vet ./...

cross-build:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/srch-darwin-arm64 ./cmd/srch
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/srch-darwin-amd64 ./cmd/srch
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/srch-linux-amd64 ./cmd/srch
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/srch-linux-arm64 ./cmd/srch
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/srch-windows-amd64.exe ./cmd/srch

clean:
	rm -f $(BINARY)
