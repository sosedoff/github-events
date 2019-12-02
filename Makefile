.PHONY: clean build

build:
	go build

clean:
	rm *.json
	rm *.log
