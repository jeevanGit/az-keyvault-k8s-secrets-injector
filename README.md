# azure-keyvault-service

Repo hosts Kubernetes Environment Injector init-container to retrieve secrets/keys from Azure KeyVault and populate them in form of environment variables for your application/container.


## Overview

This project offer the component for handling Azure Key Vault Secrets in Kubernetes:

* Azure Key Vault Env Injector

The **Azure Key Vault Env Injector** (Env Injector for short) is a Kubernetes Mutating Webhook that transparently injects Azure Key Vault secrets as environment variables into programs running in containers, without touching disk or in any other way expose the actual secret content outside the program.

The motivation behind this project was:

1. Avoid a direct program dependency on Azure Key Vault for getting secrets, and adhere to the 12 Factor App principle for configuration (https://12factor.net/config)
2. Make it simple, secure and low risk to transfer Azure Key Vault secrets into Kubernetes as native Kubernetes secrets
3. Securely and transparently be able to inject Azure Key Vault secrets as environment variables to applications, without having to use native Kubernetes secrets

Use the Env Injector if:

* any of the [risks documented with Secrets in Kubernetes](https://kubernetes.io/docs/concepts/configuration/secret/#risks) is not acceptable
* there are concerns about storing and exposing base64 encoded Azure Key Vault secrets as Kubernetes `Secret` resources
* preventing Kubernetes users to gain access to Azure Key Vault secret content is important
* the application running in the container support getting secrets as environment variables
* secret environment variable values should not be revealed to Kubernetes resources like Pod specs, stored on disks, visible in logs or exposed in any way other than in-memory for the application

## How it works

The Env Injector is developed using a Mutating Admission Webhook that triggers just before every Pod gets created. To allow cluster administrators some control over which Pods this Webhook gets triggered for, it must be enabled per namespace using the azure-key-vault-env-injection label, like in the example below:

```
apiVersion: v1
kind: Namespace
metadata:
  name: akv-test
  labels:
    azure-key-vault-env-injection: enabled
```

As with the Controller, the Env Injector relies on AzureKeyVaultSecret resources to provide information about the Azure Key Vault secrets.

The Env Injector will start processing containers containing one or more environment placeholders like below:

```
env:
- name: azurekeyvault
  value: <name of Azure KeyVault>
- name: <name of environment variable>
  value: <name of AzureKeyVaultSecret>@azurekeyvault

...
```

It will start by injecting a init-container into the Pod. This init-container copies over the azure-keyvault-env executable to a share volume between the init-container and the original container. It then changes either the CMD or ENTRYPOINT, depending on which was used by the original container, to use the azure-keyvault-env executable instead, and pass on the "old" command as parameters to this new executable. The init-container will then complete and the original container will start.

When the original container starts it will execute the azure-keyvault-env command which will download any Azure Key Vault secrets, identified by the environment placeholders above. The remaining step is for azure-keyvault-env to execute the original command and params, pass on the updated environment variables with real secret values. This way all secrets gets injected transparently in-memory during container startup, and not reveal any secret content to the container spec, disk or logs.


## Authentication

No credentials are needed for managed identity authentication. The Kubernetes cluster must be running in Azure and the aad-pod-identity controller must be installed. A AzureIdentity and AzureIdentityBinding must be defined. See https://github.com/Azure/aad-pod-identity for details.

### Custom Authentication for Env Injector

To use custom authentication for the Env Injector, set the environment variable CUSTOM_AUTH to true.

By default each Pod using the Env Injector pattern must provide their own credentials for Azure Key Vault using Authentication options below.

To avoid that, support for a more convenient solution is added where the Azure Key Vault credentials in the Env Injector (using Authentication options below) is "forwarded" to the the Pods. This is enabled by setting the environment variable CUSTOM_AUTH_INJECT to true. Env Injector will then create a Kubernetes Secret containing the credentials and modify the Pod's env section to reference the credentials in the Secret.

#### Custom Authentication Options

The following authentication options are available:

| Authentication type |	Environment variable |	Description |
| ------------------- | -------------------- | ------------ |
| Managed identities for Azure resources (used to be MSI) | | No credentials are needed for managed identity authentication. The Kubernetes cluster must be running in Azure and the `aad-pod-identity` controller must be installed. A `AzureIdentity` and `AzureIdentityBinding` must be defined. See https://github.com/Azure/aad-pod-identity for details. |
| Client credentials 	| AZURE_TENANT_ID 	   | The ID for the Active Directory tenant that the service principal belongs to. |
|                     |	AZURE_CLIENT_ID 	   | The name or ID of the service principal. |
|                     |	AZURE_CLIENT_SECRET  | The secret associated with the service principal. |
| Certificate 	      | AZURE_TENANT_ID      | The ID for the Active Directory tenant that the certificate is registered with. |
|                     | AZURE_CLIENT_ID      | The application client ID associated with the certificate. |
|                     | AZURE_CERTIFICATE_PATH | The path to the client certificate file. |
|                     | AZURE_CERTIFICATE_PASSWORD | The password for the client certificate. |
| Username/Password   | AZURE_TENANT_ID | The ID for the Active Directory tenant that the user belongs to. |
|                     | AZURE_CLIENT_ID | The application client ID. |
|                     | AZURE_USERNAME  | The username to sign in with.
|                     | AZURE_PASSWORD  | The password to sign in with. |


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
make push
```

## How to use it

First, lets build and push the test client - image `test-client:v1alpha1`

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

2. It creates empty volume `azure-keyvault-env` and mounts it to `/azure-keyvault/`

```golang
			Volumes: []apiv1.Volume{
				{
					Name: "azure-keyvault-env",
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
							Name: "azure-keyvault-env", MountPath: "/azure-keyvault/",
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
							Name:      "azure-keyvault-env",
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

Also in this step, `test-deployment` mounts same volume `azure-keyvault-env` to `/azure-keyvault/` for `test-client` container, hence now it can 'see' the binary `secret-injector` from the init container - see step 3.

5. And, finally, it executes the binary `secret-injector` from the init container and passes "application" as a parameter to it, as such:

```go
    Command:         []string{"sh", "-c", "/azure-keyvault/secret-injector /my-application-script.sh"},
```

What happens in this step, the binary `secret-injector` takes environment variable `env_secret_name=secret1@azurekeyvault` and replaces value with the secret from the vault that it points to: vault's name is `AzureKeyVault` and secret's name is `secret1`. Then, the binary `secret-injector` executes the application code, in this case it's script `my-application-script.sh`, which inherits "new" environment along with secrets populated as environment variables.
This is secure way to make the secrets as environment variables - even if someone hacks into the pod and tries to see the manifest of it, hopping to learn the secrets from the environment, all manifest would show is "old" environment variable `env_secret_name=secret1@AzureKeyVault`.




## Credits

Credit goes to Banzai Cloud for coming up with the [original idea](https://banzaicloud.com/blog/inject-secrets-into-pods-vault/) of environment injection for their [bank-vaults](https://github.com/banzaicloud/bank-vaults) solution, which they use to inject Hashicorp Vault secrets into Pods.


---
