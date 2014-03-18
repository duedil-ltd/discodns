all::   clean build
build:: compile test

clean:
	@echo "● Cleaning out the build folder"
	rm -rf build/*
	@echo "✔ Cleaned build"

compile:
	@echo "● Building to build/zoodns"
	go build -o build/zoodns *.go
	@echo "✔ Successfully built build/zoodns"

test:
	@echo -n
