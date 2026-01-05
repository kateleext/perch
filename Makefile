.PHONY: build run poc clean

build:
	go build -o perch ./cmd/perch

run:
	go run ./cmd/perch

poc:
	./poc/perch.sh

clean:
	rm -f perch

install:
	go install ./cmd/perch
