.PHONY: run build clean

run:
	go run ./cmd/server/main.go

build:
	go build -o bin/trendborse-server ./cmd/server/main.go

clean:
	rm -rf bin/
