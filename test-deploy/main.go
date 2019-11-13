

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	/*appsv1 "k8s.io/api/apps/v1"*/
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	_ "k8s.io/client-go/util/retry"
)

const (
  controllerPodName = "env-injector-pod"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

  podsClient := clientset.CoreV1().Pods(apiv1.NamespaceDefault)
  pod := &apiv1.Pod{
        ObjectMeta: metav1.ObjectMeta{
    			Name: controllerPodName,
          Labels: map[string]string{
						"aadpodidbinding": "pod-selector-label",
					},
    		},
				Spec: apiv1.PodSpec{
          InitContainers: []apiv1.Container{
						{
							Name:  "env-injector-init",
							Image: "securityopregistrytest.azurecr.io/env-injector:v1alpha1",
              Command: []string{ "sh", "-c", "cp /usr/local/bin/* /azure-keyvault/" },
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
            {
							Name:  "debug",
							Image: "teran/ubuntu-network-troubleshooting",
              ImagePullPolicy: apiv1.PullAlways,
              VolumeMounts: []apiv1.VolumeMount{
          			{
          				Name: "azure-keyvault-env", MountPath: "/azure-keyvault/",
          			},
          		},

						},

					},
					Containers: []apiv1.Container{
						{
							Name:  "test-client",
							Image: "securityopregistrytest.azurecr.io/test-client:v1alpha1",
              Command: []string{ "sh", "-c", "/azure-keyvault/env-injector sleep 6000" },
              ImagePullPolicy: apiv1.PullAlways,
              VolumeMounts: []apiv1.VolumeMount{
          			{
          				Name:      "azure-keyvault-env",
          				MountPath: "/azure-keyvault/",
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

				},
      }

	// Create Deployment
	fmt.Println("Creating Pod...")
  result_pod, err := podsClient.Create(pod)
	if err != nil {

    fmt.Println(err)

    fmt.Println("Trying to Delete Pod...")
  	deletePolicy := metav1.DeletePropagationForeground
  	if err := podsClient.Delete(controllerPodName, &metav1.DeleteOptions{
  		PropagationPolicy: &deletePolicy,
  	}); err != nil {
  		panic(err)
  	}
  	fmt.Println("Deleted Pod.")

	}
	fmt.Printf("Created pod %q.\n", result_pod.GetObjectMeta().GetName())

/*
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	// Update Deployment
	prompt()
	fmt.Println("Updating deployment...")
	//    You have two options to Update() this Deployment:
	//
	//    1. Modify the "deployment" variable and call: Update(deployment).
	//       This works like the "kubectl replace" command and it overwrites/loses changes
	//       made by other clients between you Create() and Update() the object.
	//    2. Modify the "result" returned by Get() and retry Update(result) until
	//       you no longer get a conflict error. This way, you can preserve changes made
	//       by other clients between Create() and Update(). This is implemented below
	//			 using the retry utility package included with client-go. (RECOMMENDED)
	//
	// More Info:
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, getErr := deploymentsClient.Get("env-injector-deployment", metav1.GetOptions{})
		if getErr != nil {
			panic(fmt.Errorf("Failed to get latest version of Deployment: %v", getErr))
		}

		result.Spec.Replicas = int32Ptr(1)
		result.Spec.Template.Spec.Containers[0].Image = "securityopregistrytest.azurecr.io/env-injector:v1alpha1"
		_, updateErr := deploymentsClient.Update(result)
		return updateErr
	})
	if retryErr != nil {
		panic(fmt.Errorf("Update failed: %v", retryErr))
	}
	fmt.Println("Updated deployment...")
*/

	// List Deployments
	prompt()
	fmt.Printf("Listing deployments in namespace %q:\n", apiv1.NamespaceDefault)
	list, err := podsClient.List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, d := range list.Items {
		fmt.Printf(" * %s \n", d.Name)
	}

	// Delete Deployment
	prompt()
  fmt.Println("Deleting Pod...")
	deletePolicy := metav1.DeletePropagationForeground
	if err := podsClient.Delete(controllerPodName, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		panic(err)
	}
	fmt.Println("Deleted Pod.")

/*
	fmt.Println("Deleting deployment...")
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete("env-injector-deployment", &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		panic(err)
	}
	fmt.Println("Deleted deployment.")
*/

}

func prompt() {
	fmt.Printf("-> Press Return key to continue.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		break
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	fmt.Println()
}

func int32Ptr(i int32) *int32 { return &i }
