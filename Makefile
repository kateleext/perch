.PHONY: build run poc clean

build:
	go build -o drift ./cmd/drift

run:
	go run ./cmd/drift

poc:
	./poc/drift.sh

clean:
	rm -f drift

install:
	go install ./cmd/drift
