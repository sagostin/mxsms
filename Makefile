BUILD_DATE    := `date -u +%Y-%m-%d`
BUILD_GIT     := `git rev-parse --short HEAD`
FLAGS         := -ldflags "-X main.build=$(BUILD_GIT) -X main.date=$(BUILD_DATE)"

.PHONY: build debug run release

run: debug
	@echo "run..."
	@./mxsms2 -debug

debug:
	@echo "debug build..."
	@go build -race $(FLAGS)

build:
	@echo "build..."
	@go build $(FLAGS)

release:
	@echo "build release..."
	@GOOS=linux GOARCH=amd64 go build $(FLAGS) -o mxsms-linux-amd64
	@GOOS=linux GOARCH=386 go build $(FLAGS) -o mxsms-linux-386
	@GOOS=darwin GOARCH=amd64 go build $(FLAGS) -o mxsms-darwin-amd64
	@zip -9 mxsms-0.5.1.zip mxsms-linux-amd64 mxsms-linux-386 mxsms-darwin-amd64 config.yaml