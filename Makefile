GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GIT_COMMIT=$(shell git log -1 --pretty=format:"%H %cI" 2>/dev/null)

default: build

show-commit:
	@echo ${GIT_COMMIT}

update_submodule:
	git submodule sync

build: update_submodule
	$(GOBUILD) -ldflags="-X 'main.codeVersion=$(GIT_COMMIT)'" -o ./cesium_tiler ./*.go

clean:
	$(GOCLEAN)
	rm -rf ./cesium_tiler
