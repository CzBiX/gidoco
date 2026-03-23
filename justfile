go_build := "CGO_ENABLED=0 go build -trimpath -ldflags '-w -s'"

default:
    @just --list

build:
    mkdir -p build
    {{go_build}} -o build/gidoco .

build-debug:
    CGO_ENABLED=0 go build -gcflags="all=-N -l" -o build/gidoco .

test:
    go test -v ./...

test-cover:
    mkdir -p build
    go test -coverprofile=build/coverage.out ./...
    go tool cover -html build/coverage.out -o build/coverage.html
    @echo "Coverage report: build/coverage.html"

lint:
    go vet ./...

tidy:
    go mod tidy

clean:
    rm -rf build
