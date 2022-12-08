.PHONY: clean build

build:
	go build

clean:
	rm -f *.json
	rm -f *.log
	rm -f ./bin/*

release:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./bin/github-events_linux_amd64
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o ./bin/github-events_darwin_amd64
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o ./bin/github-events_darwin_arm64
