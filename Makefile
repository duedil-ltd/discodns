all::   clean build
build:: compile test

clean:
	@echo "\033[34m●\033[39m Cleaning out the build folder"
	rm -rf build/*
	@echo "\033[32m✔\033[39m Cleaned build"

compile:
	@echo "\033[34m●\033[39m Building to build/etcdns"
	go build -o build/etcdns *.go
	@echo "\033[32m✔\033[39m Successfully built build/etcdns"

test:
	@echo -n
