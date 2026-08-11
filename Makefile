BINARY := memr

.PHONY: build test lint install clean

build:
	go build -o $(BINARY) .

test:
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	go test ./...

lint:
	go vet ./...

install:
	go install .

clean:
	rm -f $(BINARY)
	rm -rf dist/
