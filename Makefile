.PHONY: build test fmt fmt-check vet clean ci install

build:
	go build ./...

install:
	go install ./cmd/homebase/

test:
	go test -v -race ./...

fmt:
	gofmt -w .

fmt-check:
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "unformatted files:"; echo "$$unformatted"; exit 1; fi; echo "✓ fmt ok"

vet:
	go vet ./...

prove:
	@if [ ! -f tla2tools.jar ]; then wget -qO- https://github.com/tlaplus/tlaplus/releases/download/v1.8.0/tla2tools.jar > tla2tools.jar; fi
	java -cp tla2tools.jar tlc2.TLC verification/homebase.tla
	@echo "✓ formal TLA+ bounds verified"

ci: fmt-check vet test prove

clean:
	go clean
	rm -f tla2tools.jar
