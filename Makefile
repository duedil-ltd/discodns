all::   clean build
build:: compile test

clean:
	@echo "● Cleaning out the build folder"
	rm -rf build/*
	@echo "✔ Cleaned build"

compile:
	@echo "● Building to build/duedil-dns"
	go build -o build/duedil-dns *.go
	@echo "✔ Successfully built build/duedil-dns"

test:
	@echo -n
