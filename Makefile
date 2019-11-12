COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GOOS?=linux
GOARCH?=amd64
PROJECT?=

include vars-az.mk

APP?=env-injector
PORT?=8001
APIVER?=v1
RELEASE?=v1alpha1
#IMAGE?=securityopregistrytest.azurecr.io/env_injector
IMAGE?=${DOCKER_ORG}/${APP}:${RELEASE}
IMAGE_derived?=${DOCKER_ORG}/${APP}-derived:${RELEASE}
ENV?=DEV
K8S_CHART?=azure-keyvault-secrets
K8S_NAMESPACE?=app1-ns

helm:
		kubectl create serviceaccount --namespace kube-system tiller
		kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
		helm init --service-account tiller --upgrade

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
		docker run --name ${APP} -p ${PORT}:${PORT} --rm \
			-e "PORT=${PORT}" \
			$(IMAGE)

push: container
		docker push $(IMAGE)
		docker push $(IMAGE_derived)

container: build
		-docker build -t $(IMAGE) .
		-docker build -t $(IMAGE_derived) ./combined
		cp ./bin/${APP} ./combined
		rm -f ./bin/${APP}

deployclean:
		helm del --purge "${K8S_CHART}-${K8S_NAMESPACE}"

deploy:
		for t in $(shell find ./charts/${K8S_CHART} -type f -name "values-template.yaml"); do \
					cat $$t | \
						sed -E "s/{{ .ServiceName }}/$(APP)/g" | \
						sed -E "s/{{ .Release }}/$(RELEASE)/g" | \
						sed -E "s/{{ .Env }}/$(ENV)/g" | \
						sed -E "s/{{ .Kube_namespace }}/$(K8S_NAMESPACE)/g" | \
						sed -E "s/{{ .ApiVer }}/$(APIVER)/g" | \
						sed -E "s/{{ .LBPort }}/$(LB_EXTERNAL_PORT)/g" | \
						sed -E "s/{{ .ContainerPort }}/$(PORT)/g" | \
						sed -E "s/{{ .DockerOrg }}/$(DOCKER_ORG)/g"; \
		done > ./charts/${K8S_CHART}/values.yaml
		helm install --name "${K8S_CHART}-${K8S_NAMESPACE}" --values ./charts/${K8S_CHART}/values.yaml --namespace ${K8S_NAMESPACE}  ./charts/${K8S_CHART}/
		echo "Cleaning up temp files.."
		kubectl get services --all-namespaces | grep ${APP}
		echo $(kubectl get pods -n $namespace -l app=$(IMAGE) -o jsonpath='{.items[].metadata.name}')

# echo "Cleaning up temp files.." && rm ./charts/${K8S_CHART}/values.yaml

.PHONY: glide
glide:
ifeq ($(shell command -v glide 2> /dev/null),)
		curl https://glide.sh/get | sh
endif

.PHONY: deps
deps: glide
		glide install
