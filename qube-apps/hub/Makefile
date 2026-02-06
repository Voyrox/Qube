.PHONY: build run clean install

build:
	go build -o qube-hub main.go

run:
	go run main.go

clean:
	rm -f qube-hub
	rm -rf storage/

install:
	go install

deps:
	go mod download

test:
	go test ./...
