COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GOOS?=linux
GOARCH?=amd64
PROJECT?=

include vars-az.mk

APP?=test-client
PORT?=8001
APIVER?=v1
RELEASE?=v1alpha1
IMAGE?=${DOCKER_ORG}/${APP}:${RELEASE}
ENV?=DEV
K8S_NAMESPACE?=app1-ns

helm:
		kubectl create serviceaccount --namespace kube-system tiller
		kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
		helm init --service-account tiller --upgrade

run: container
		docker stop ${APP} || true && docker rm ${APP} || true
		docker run --name ${APP} -p ${PORT}:${PORT} --rm \
			-e "PORT=${PORT}" \
			$(IMAGE)

push: container
		docker push $(IMAGE)

container:
		docker build -t $(IMAGE) .

deployclean:
		helm del --purge "${K8S_CHART}-${K8S_NAMESPACE}"

.PHONY: glide
glide:
ifeq ($(shell command -v glide 2> /dev/null),)
		curl https://glide.sh/get | sh
endif

.PHONY: deps
deps: glide
		glide install
