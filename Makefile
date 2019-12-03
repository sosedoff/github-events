.PHONY: clean build

build:
	go build

clean:
	rm -f *.json
	rm -f *.log

release:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./bin/github-events_linux_amd64
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o ./bin/github-events_darwin_amd64