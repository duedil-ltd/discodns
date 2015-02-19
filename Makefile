all::   clean build
build:: get compile

clean:
	@echo "\033[34m●\033[39m Cleaning out the build folder ./build"
	rm -rf build/*
	@echo "\033[32m✔\033[39m Cleaned ./build"

get:
	@echo "\033[34m●\033[39m Downloading go packages"
	go get github.com/tools/godep
	go get -d
	godep restore
	@echo "\033[32m✔\033[39m Finished downloading packages"

compile:
	@echo "\033[34m●\033[39m Building into ./build"
	mkdir -p build/bin
	go build -o build/bin/discodns *.go
	@echo "\033[32m✔\033[39m Successfully built into ./build"

test:
	@echo "\033[34m●\033[39m Running tests"
	go test
	@echo "\033[32m✔\033[39m Tests passed"

install:
	@echo "\033[34m●\033[39m Installing into /usr/local/bin"
	cp build/bin/discodns /usr/local/bin/
	@echo "\033[32m✔\033[39m Successfully installed into /usr/local/bin/discodns"
