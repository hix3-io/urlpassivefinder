BIN      := urlpassivefinder
GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: all build test vet lint clean install

all: vet test build

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/urlpassivefinder

test:
	go test -race ./...

vet:
	go vet ./...

lint: vet

install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/urlpassivefinder

clean:
	rm -f $(BIN)
