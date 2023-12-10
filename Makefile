GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
CGO_LDFLAGS="-g -O2 -lm"
GIT_COMMIT=$(shell git log -1 --pretty=format:"%H %cI" 2>/dev/null)


default: build

show-commit:
	@echo ${GIT_COMMIT}

update_submodule:
	git submodule sync

build: update_submodule
	${GOCMD} mod tidy
	CGO_LDFLAGS=$(CGO_LDFLAGS) $(GOBUILD) -ldflags="-X 'main.codeVersion=$(GIT_COMMIT)'" -o ./cesium_tiler ./*.go

clean:
	$(GOCLEAN)
	rm -rf ./cesium_tiler
