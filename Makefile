.PHONY: dev build build-windows test vet check

dev:            ## run with hot reload
	wails dev

build:          ## native build for this OS
	wails build

build-windows:  ## cross-compile for Windows from any OS (no CGO needed)
	wails build -platform windows/amd64

test:           ## unit tests with the race detector
	go test -race ./...

vet:            ## vet native and Windows targets
	go vet ./...
	GOOS=windows CGO_ENABLED=0 go vet -tags desktop,production ./...

check: vet test
