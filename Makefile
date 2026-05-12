.PHONY: all build test test-race c-shared c-test c-example clean

all: build test

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

c-shared:
	cd cgo && go build -buildmode=c-shared -o libonce.so

c-example: c-shared
	gcc -o examples/example examples/example.c -Lcgo -lonce -Wl,-rpath,cgo

c-test: c-shared
	gcc -o examples/test examples/test.c -Lcgo -lonce -Wl,-rpath,cgo -lpthread

clean:
	rm -f cgo/libonce.so cgo/libonce.h examples/example examples/test
	rm -f coverage.out coverage.html
