.PHONY: lint test vendor clean yaegi_test

export GO111MODULE=on

default: lint test

lint:
	golangci-lint run

test:
	go test -v -cover ./...

yaegi_test:
	rm -rf ./tmp
	mkdir -p ./tmp/src/github.com/NX211/traefik-proxmox-provider
	cp -r ./internal ./provider ./vendor ./*.go ./.traefik.yml ./go.mod ./go.sum ./tmp/src/github.com/NX211/traefik-proxmox-provider/
	GOPATH=$(shell pwd)/tmp yaegi test github.com/NX211/traefik-proxmox-provider
	rm -rf ./tmp

vendor:
	go mod vendor

clean:
	rm -rf ./vendor
	rm -rf ./tmp