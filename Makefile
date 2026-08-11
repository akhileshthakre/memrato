BINARY := memrato

.PHONY: build test lint install npm clean

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

npm:
	node scripts/build-npm.mjs $(VERSION)

clean:
	rm -f $(BINARY)
	rm -rf dist/ npm-dist/
