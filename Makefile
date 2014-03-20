all::   clean build
build:: compile test

clean:
	@echo "\033[34m●\033[39m Cleaning out the build folder ./build"
	rm -rf build/*
	@echo "\033[32m✔\033[39m Cleaned ./build"

compile:
	@echo "\033[34m●\033[39m Building into ./build"
	mkdir -p build/bin
	go build -o build/bin/etcdns *.go
	@echo "\033[32m✔\033[39m Successfully built into ./build"

test:
	@echo -n
