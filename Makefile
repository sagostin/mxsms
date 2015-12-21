BUILD_DATE := `date -u +%Y-%m-%d`
BUILD_GIT  := `git rev-parse --short HEAD`
FLAGS      := -ldflags "-X main.build=$(BUILD_GIT) -X main.date=$(BUILD_DATE)"

.PHONY: build debug run

run: debug
	@echo "run..."
	@./mxsms -debug

debug:
	@echo "debug build..."
	@go build -race $(FLAGS)

build:
	@echo "build..."
	@go build $(FLAGS)