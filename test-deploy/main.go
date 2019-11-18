

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
  controllerPodName = "application-pod"
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

	podsClient, pod := clientset.CoreV1().Pods(apiv1.NamespaceDefault), &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerPodName,
			Labels: map[string]string{
				"aadpodidbinding": "pod-selector-label",
			},
		},
		Spec: apiv1.PodSpec{
			InitContainers: []apiv1.Container{
				{
					Name:            "secret-injector-init",
					Image:           "securityopregistrytest.azurecr.io/secret-injector:v1alpha1",
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
				{
					Name:            "debug",
					Image:           "teran/ubuntu-network-troubleshooting",
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
					Name:            "test-client",
					Image:           "securityopregistrytest.azurecr.io/test-client:v1alpha1",
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
						{Name: "SECRET_INJECTOR_SECRET_NAME_1", Value: "secret1",},
						{Name: "SECRET_INJECTOR_MOUNT_PATH_1", Value: "/etc/secrets",},
						{Name: "SECRET_INJECTOR_SECRET_NAME_secret2", Value: "secret1",},
						{Name: "SECRET_INJECTOR_MOUNT_PATH_secret2", Value: "/etc/secrets",},
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
