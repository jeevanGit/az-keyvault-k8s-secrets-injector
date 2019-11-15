COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GOOS?=linux
GOARCH?=amd64
PROJECT?=

include vars-az.mk

APP?=secret-injector
APIVER?=v1
RELEASE?=v1alpha1
IMAGE?=${DOCKER_ORG}/${APP}:${RELEASE}
ENV?=DEV


clean:
		rm -f ./bin/${APP}

build: clean
		echo "GOPATH: " ${GOPATH}
		CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
			-ldflags "-s -w -X version.Release=${RELEASE} \
			-X version.Commit=${COMMIT} -X version.BuildTime=${BUILD_TIME}" \
			-o ./bin/${APP}

run: container
		docker stop ${APP} || true && docker rm ${APP} || true
		docker run --name ${APP} --rm \
			$(IMAGE)

push: container
		docker push $(IMAGE)

container: build
		-docker build -t $(IMAGE) .
		rm -f ./bin/${APP}

.PHONY: glide
glide:
ifeq ($(shell command -v glide 2> /dev/null),)
		curl https://glide.sh/get | sh
endif

.PHONY: deps
deps: glide
		glide install
