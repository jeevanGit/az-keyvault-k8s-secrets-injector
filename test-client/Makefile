COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GOOS?=linux
GOARCH?=amd64
PROJECT?=

include ../vars-az.mk

APP?=test-client
APIVER?=v1
RELEASE?=v1alpha1
IMAGE?=${DOCKER_ORG}/${APP}:${RELEASE}
ENV?=DEV


run: container
		docker stop ${APP} || true && docker rm ${APP} || true
		docker run --name ${APP} --rm \
			$(IMAGE)

push: container
		docker push $(IMAGE)

container:
		-docker build -t $(IMAGE) .

.PHONY: glide
glide:
ifeq ($(shell command -v glide 2> /dev/null),)
		curl https://glide.sh/get | sh
endif

.PHONY: deps
deps: glide
		glide install
