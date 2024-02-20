.PHONY: install tests release build

install:
	go install ./...

tests:
	go test -v ./...

release:
	goreleaser release

build:
	go build -o sndbot
