BIN      := urlpassivefinder
GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: all build test vet lint clean install

all: vet test build

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test -race ./...

vet:
	go vet ./...

lint: vet

install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" .

clean:
	rm -f $(BIN)
