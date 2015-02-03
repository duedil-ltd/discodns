all::   clean build test
build:: get compile

motd:
	@echo
	@echo '       ___                     __'
	@echo '  ____/ (_)_____________  ____/ /___  _____'
	@echo ' / __  / / ___/ ___/ __ \/ __  / __ \/ ___/'
	@echo '/ /_/ / (__  ) /__/ /_/ / /_/ / / / (__  )'
	@echo '\__,_/_/____/\___/\____/\__,_/_/ /_/____/'
	@echo
	@echo '© Copyright DueDil 2015. Licensed under MIT.'
	@echo

clean: motd
	@echo "\033[34m●\033[39m Cleaning out the build folder ./build"
	rm -rf build/*
	@echo "\033[32m✔\033[39m Cleaned ./build"

get: motd
	@echo "\033[34m●\033[39m Downloading go packages"
	go get -d
	@echo "\033[32m✔\033[39m Finished downloading packages"

compile: motd get
	@echo "\033[34m●\033[39m Building into ./build"
	mkdir -p build/bin
	go build -o build/bin/discodns *.go
	@echo "\033[32m✔\033[39m Successfully built into ./build"

test: motd
	@echo "\033[34m●\033[39m Running tests"
	go test -race ./
	@echo "\033[32m✔\033[39m Tests passed"

install: motd compile
	@echo "\033[34m●\033[39m Installing into /usr/local/bin"
	cp build/bin/discodns /usr/local/bin/
	@echo "\033[32m✔\033[39m Successfully installed into /usr/local/bin/discodns"
