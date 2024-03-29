# Kubernetes Secrets Injector

Repo hosts Kubernetes Secrets Injector init-container to retrieve secrets/keys from Azure KeyVault and populate them in form of environment variables as well as secrets files for your application/container.


## Overview

This project offer the component for handling Azure Key Vault Secrets in Kubernetes:

* Azure Key Vault Secrets Injector

The **Azure Key Vault Secrets Injector** (Secrets Injector for short) is a Kubernetes Mutating Webhook that transparently injects Azure Key Vault secrets as environment variables into programs running in containers, without touching disk or in any other way expose the actual secret content outside the program.

The motivation behind this project was:

1. Avoid a direct program dependency on Azure Key Vault for getting secrets, and adhere to the 12 Factor App principle for configuration (https://12factor.net/config)
2. Make it simple, secure and low risk to transfer Azure Key Vault secrets into Kubernetes as native Kubernetes secrets.
3. Securely and transparently be able to inject Azure Key Vault secrets as files and environment variables to applications, without having to use native Kubernetes secrets.

Use the Secrets Injector if:

* any of the [risks documented with Secrets in Kubernetes](https://kubernetes.io/docs/concepts/configuration/secret/#risks) is not acceptable
* there are concerns about storing and exposing base64 encoded Azure Key Vault secrets as Kubernetes `Secret` resources
* preventing Kubernetes users to gain access to Azure Key Vault secret content is important
* the application running in the container support getting secrets as environment variables and as a files
* secret environment variable values should not be revealed to Kubernetes resources like Pod specs, stored on disks, visible in logs or exposed in any way other than in-memory for the application

## How it works

The Secrets Injector will start processing containers containing one or more environment placeholders like below:

```
env:
- name: azurekeyvault
  value: <name of Azure KeyVault>
- name: <name of environment variable>
  value: <name of AzureKeyVaultSecret>@azurekeyvault

...
```

It will start by injecting a init-container into the Pod. This init-container copies over the `secret-injector` executable to a share volume between the init-container and the original container. It then changes either the CMD or ENTRYPOINT, depending on which was used by the original container, to use the `secret-injector` executable instead, and pass on the "old" command as parameters to this new executable. The init-container will then complete and the original container will start.

When the original container starts it will execute the `secret-injector` command which will download any Azure Key Vault secrets, identified by the environment placeholders above. The remaining step is for `secret-injector` to execute the original command and params, pass on the updated environment variables with real secret values. This way all secrets gets injected transparently in-memory during container startup, and not reveal any secret content to the container spec, disk or logs.


## Authentication

No credentials are needed for managed identity authentication. The Kubernetes cluster must be running in Azure and the aad-pod-identity controller must be installed. A AzureIdentity and AzureIdentityBinding must be defined.
See https://github.com/Azure/aad-pod-identity for details.

In context of test client deployment, designed for the purpose of testing Secrets Injector, it offers pre-build labels for aad-pod-identity selector

```go
	podsClient, pod := clientset.CoreV1().Pods(apiv1.NamespaceDefault), &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerPodName,
			Labels: map[string]string{
				"aadpodidbinding": "pod-selector-label",
			},
		},
```

Assuming `AzureIdentity` created with the name `app1-principal`

```yaml
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentity
metadata:
  name: app1-principal
spec:
  type: 0
  ResourceID: <your ManagedIdentity Resource ID>
  ClientID: <your ManagedIdentity ClientID>
```

Along with `AzureIdentityBinding` defined as

```yaml
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentityBinding
metadata:
  name: app1-principal-binding
spec:
  AzureIdentity: app1-principal
  Selector:  pod-selector-label
```

It is evident that only pods label as `aadpodidbinding=pod-selector-label` would be assigned with AAD Pod Identity and have access to selected Key Vault.


### Custom Authentication for Secrets Injector

To use custom authentication for the Secrets Injector, set the environment variable CUSTOM_AUTH to true.

By default each Pod using the Secrets Injector pattern must provide their own credentials for Azure Key Vault using Authentication options below.

To avoid that, support for a more convenient solution is added where the Azure Key Vault credentials in the Secrets Injector(using Authentication options below) is "forwarded" to the the Pods. This is enabled by setting the environment variable CUSTOM_AUTH_INJECT to true. Secrets Injectorwill then create a Kubernetes Secret containing the credentials and modify the Pod's env section to reference the credentials in the Secret.


## Authorization

Authenticated account will need get permissions to the different object types in Azure Key Vault.

Note: It's only possible to control access at the top level of Azure Key Vault, not per object/resource. The recommendation is therefore to have a dedicated Key Vault per cluster.

Access is controlled through Azure Key Vault policies and can be configured through Azure CLI like this:

Azure Key Vault Secrets:

```
az keyvault set-policy -n <azure key vault name> --secret-permissions get --spn <service principal id> --subscription <azure subscription>
```

Azure Key Vault Keys:

```
az keyvault set-policy -n <azure key vault name> --key-permissions get --spn <service principal id> --subscription <azure subscription>
```

**Note:**

To allow cluster administrators some control over which Pods this Webhook gets triggered for, it must be enabled per namespace using the `azure-key-vault-env-injection` label, like in the example below:

```
apiVersion: v1
kind: Namespace
metadata:
  name: akv-test
  labels:
    azure-key-vault-env-injection: enabled
```


## Build Environment Injector

Make sure you create `vars-az.mk` file and define `DOCKER_ORG`:

```
DOCKER_ORG?=<your acr name>.azurecr.io
```

Also, in `Makefile`, set variables `APP` and `RELEASE`

```
APP?=secret-injector
RELEASE?=v1alpha1
```

Then, login into the instance of ACR:

```
az acr login --name <your acr name>
```

To build the binaries run:

```
make build
```

This step compiles binary and places it under `./bin` directory.

Then, build image `secret-injector:v1alpha1` and push it to ACR instance:

```
export GOPATH=$GOPATH:$(pwd)
make push
```

# How to use it

Assuming all instructions from previous section 'Build Environment Injector' are completed.
Lets by building and pushing the test client - image `test-client:v1alpha1`

```
cd test-client
make push
```

Second, build and push the test deployment-pod ,image called `test-deployment:v1alpha1`, what is does it simulates the controller (implements Kubernetes Mutating Webhook) that ingests init-container into your application container to set environment variables based on the secrets from the vault you specify.

```
cd ../test-deploy
make push
```

At this point, there should be 3 images in total: `test-client:v1alpha1`, `test-deployment:v1alpha1` and `secret-injector:v1alpha1`


By looking at `fake-controller.yaml` it should be evident that it takes `<your registry>/test-deployment:v1alpha1` image and creates a pod, which contains binary `test-deployment` was built in previous step.
Source code of binary `test-deployment` located at [./test-deploy/main.go](./test-deploy/main.go), along with corresponding [./test-deploy/Dockerfile](./test-deploy/Dockerfile)

Next step is to execute the test deployment binary:

```bash
kubectl exec -it fake-controller -- /usr/local/bin/test-deployment
```

What binary `test-deployment` does is set of following steps:

1. Creates pod named `application-pod` which simulates a pod created by an application

```golang
	podsClient, pod := clientset.CoreV1().Pods(apiv1.NamespaceDefault), &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerPodName,
			Labels: map[string]string{
				"aadpodidbinding": "pod-selector-label",
			},
		},
...

```

2. It creates empty volume ``secret-injector`` and mounts it to `/azure-keyvault/`

```golang
			Volumes: []apiv1.Volume{
				{
					Name: "`secret-injector`",
					VolumeSource: apiv1.VolumeSource{
						EmptyDir: &apiv1.EmptyDirVolumeSource{
							Medium: apiv1.StorageMediumMemory,
						},
					},
				},
			},
```

3. Injects init container `secret-injector-init` with image from the first step `secret-injector:v1alpha1` and it copies binary `secret-injector` from `/usr/local/bin/` to mounted volume `/azure-keyvault/`

```golang
			InitContainers: []apiv1.Container{
				{
					Name:            "secret-injector-init",
					Image:           "<my-registry>/secret-injector:v1alpha1",
					Command:         []string{"sh", "-c", "cp /usr/local/bin/* /azure-keyvault/"},
					ImagePullPolicy: apiv1.PullAlways,
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name: "`secret-injector`", MountPath: "/azure-keyvault/",
						},
					},
					Env: []apiv1.EnvVar{
						{
							Name: "AzureKeyVault", Value: "aks-AC0001-keyvault",
						},
						{
							Name: "env_secret_name", Value: "secret1@AzureKeyVault",
						},
						{
							Name: "debug", Value: "true",
						},
					},
				},
			},
```


4. Then, it creates container named `test-client` where we run actual application [./test-client/my-application-script.sh](./test-client/my-application-script.sh)

```go
			Containers: []apiv1.Container{
				{
					Name:            "test-client",
					Image:           "<my-registry>/test-client:v1alpha1",
					Command:         []string{"sh", "-c", "/azure-keyvault/secret-injector /my-application-script.sh"},
					ImagePullPolicy: apiv1.PullAlways,
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "`secret-injector`",
							MountPath: "/azure-keyvault/",
						},
					},
					Env: []apiv1.EnvVar{
						{Name: "AzureKeyVault", Value: "aks-AC0001-keyvault",},
						{Name: "env_secret_name", Value: "secret1@AzureKeyVault",},
						{Name: "debug", Value: "true",},
						{Name: "SECRET_INJECTOR_SECRET_NAME_secret1", Value: "secret1",},
						{Name: "SECRET_INJECTOR_MOUNT_PATH_secret1", Value: "/etc/secrets",},
						{Name: "SECRET_INJECTOR_SECRET_NAME_secret2", Value: "secret1",},
						{Name: "SECRET_INJECTOR_MOUNT_PATH_secret2", Value: "/etc/secrets",},
					},
				},
			},
```

As it shown in the code snippet above, `test-client` take a bunch of environment variables - note these variables for the following steps.

Also in this step, `test-deployment` mounts same volume ``secret-injector`` to `/azure-keyvault/` for `test-client` container, hence now it can 'see' the binary `secret-injector` from the init container - see step 3.

5. And, finally, it executes the binary `secret-injector` from the init container and passes "application" as a parameter to it, as such:

```go
    Command:         []string{"sh", "-c", "/azure-keyvault/secret-injector /my-application-script.sh"},
```

What happens in this step:

**Part 1 - setting secrets as environment variables**

 - the binary reads environment variable `AzureKeyVault` and uses its value to set the name of Azure Key Vault, in the example above it's `aks-AC0001-keyvault`
 - takes environment variable `env_secret_name=secret1@AzureKeyVault` and retrieves the value of the secret from the vault `aks-AC0001-keyvault` and secret's name is `secret1`
 - assigned environment variable `env_secret_name` the value of the secret `secret1`
 - then, it executes the application code (in this case it's script `my-application-script.sh`) which inherits "new" environment along with secrets populated as environment variables.

**Part 2 - populating the secrets as a files**

 - the binary takes environment variable `SECRET_INJECTOR_SECRET_NAME_secret1` and follows the steps from Part 1 to retrieve the actual secret from Azure Key Vault (`AzureKeyVault`)
 - it takes variable `SECRET_INJECTOR_MOUNT_PATH_secret1` and creates read-only file `/etc/secrets/secret1` with teh content from secret `secret1`.


**This is most secure way to make the secrets as environment variables - even in the case of event hacking into the pod to reveal teh secrets by examening the manifest of the pod, all manifest would show is "old" environment variable `env_secret_name=secret1@AzureKeyVault` etc.**




## Credits

Credit goes to Banzai Cloud for coming up with the [original idea](https://banzaicloud.com/blog/inject-secrets-into-pods-vault/) of environment injection for their [bank-vaults](https://github.com/banzaicloud/bank-vaults) solution, which they use to inject Hashicorp Vault secrets into Pods.


---
