BINARY   := rethread
MODULE   := github.com/ChaitanyaPinapaka/rethread
VERSION  := 0.1.0
LDFLAGS  := -s -w -X main.version=$(VERSION)

.PHONY: build install clean tidy test

build: tidy
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: tidy
	go install -ldflags "$(LDFLAGS)" .

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

test:
	go test ./...

# Cross-compile for release
.PHONY: release
release: tidy
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=linux  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .
