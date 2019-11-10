package main

import (
	_ "encoding/json"
	"fmt"
	"github.com/spf13/viper"
	_ "io/ioutil"
	"context"
	"flag"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strings"
	"syscall"
	"os/exec"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	compute "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"

	_ "github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	_ "github.com/Azure/go-autorest/autorest/adal"

	discovery "github.com/gkarthiks/k8s-discovery"

	log "github.com/sirupsen/logrus"
)

var (
	serviceAppName    string
	vaultName 				string
	armAuthorizer     autorest.Authorizer
	k8s *discovery.K8s
)

type OAuthGrantType int
const (
	logPrefix    = "env-injector:"
	OAuthGrantTypeServicePrincipal OAuthGrantType = iota
	OAuthGrantTypeDeviceFlow
)
//------------------------------------------------------------------------------

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	log.SetLevel(log.DebugLevel)

	viper.SetConfigName("config")
	viper.AddConfigPath(".") ; viper.AddConfigPath("/")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	serviceAppName = viper.GetString("service.name")
}

/*
func grantType() OAuthGrantType {
	if config.UseDeviceFlow() {
		return OAuthGrantTypeDeviceFlow
	}
	return OAuthGrantTypeServicePrincipal
}

func getAuthorizerForResource(grantType OAuthGrantType, resource string) (autorest.Authorizer, error) {
	var a autorest.Authorizer
	var err error

	switch grantType {
	case OAuthGrantTypeServicePrincipal:
		oauthConfig, err := adal.NewOAuthConfig(
			config.Environment().ActiveDirectoryEndpoint, config.TenantID())
		if err != nil {
			return nil, err
		}

		token, err := adal.NewServicePrincipalToken(
			*oauthConfig, config.ClientID(), config.ClientSecret(), resource)
		if err != nil {
			return nil, err
		}
		a = autorest.NewBearerAuthorizer(token)

	case OAuthGrantTypeDeviceFlow:
		deviceconfig := auth.NewDeviceFlowConfig(config.ClientID(), config.TenantID())
		deviceconfig.Resource = resource
		a, err = deviceconfig.Authorizer()
		if err != nil {
			return nil, err
		}
	default:
		return a, fmt.Errorf("invalid grant type specified")
	}
	return a, err
}

func GetResourceManagementAuthorizer() (autorest.Authorizer, error) {
	if armAuthorizer != nil {
		return armAuthorizer, nil
	}

	var a autorest.Authorizer
	var err error

	a, err = getAuthorizerForResource(
		grantType(), config.Environment().ResourceManagerEndpoint)

	if err == nil {
		// cache
		armAuthorizer = a
	} else {
		// clear cache
		armAuthorizer = nil
	}
	return armAuthorizer, err
}
*/

func main() {
	log.Debugf("%s Starting azure key vault env injector", logPrefix)
	os.Setenv("CUSTOM_AUTH_INJECT", "true")
	//os.Setenv("AZURE_TENANT_ID", viper.GetString("creds.AZURE_TENANT_ID"))
	//os.Setenv("AZURE_CLIENT_ID", viper.GetString("creds.AZURE_CLIENT_ID"))
	//os.Setenv("AZURE_CLIENT_SECRET", viper.GetString("creds.AZURE_CLIENT_SECRET"))
/*
	resourcesClient := resources.NewClient( "e8eda420-4fa9-4956-923e-864018753169" )
	a, _ := kvauth.NewAuthorizerFromEnvironment()
	resourcesClient.Authorizer = a
	list, err := resourcesClient.ListComplete( context.Background(),
			"$filter=tagName eq 'az-keyvault-tag-AC0001'",
			"",
			nil,
		)
	log.Info( list.Value() )
*/

	k8s, _ = discovery.NewK8s()
	namespace, _ := k8s.GetNamespace()
	config,err := rest.InClusterConfig()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	pods, err := clientset.CoreV1().Pods( namespace ).List( metav1.ListOptions{} )
	if err != nil {
		log.Errorf("can't get list of pods: %v\n", err)
	}
	for _, pod := range pods.Items {
		log.Infof(">> %s\n", pod.Name)
	}


	rc := "secrets-operator-RG"
	subId := "e8eda420-4fa9-4956-923e-864018753169"
	az_authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
	    log.Errorf("failed NewAuthorizerFromEnvironment: %+v", az_authorizer)
	}
	vmClient := compute.NewVirtualMachinesClient(subId)
	vmClient.Authorizer = az_authorizer
	vmlist, err := vmClient.List( context.Background(), rc )
	log.Infof( ">> %s\n", vmlist.Values() )


	authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Errorf("unable to create vault authorizer: %v\n", err)
	}else{
		log.Info("Starting authorizer...")
	}
	vaultClient := keyvault.New()
	vaultClient.Authorizer = authorizer


	log.Info("<<<<<<< Environment BEFORE >>>>>>>>>")
	for _, pair := range os.Environ() {  log.Debug( pair )  }

	// Parse env variables and populate env vars from keyvault
	environ := os.Environ()
	for _, pair := range environ {
		envname, envvar := parseKeyVaultVariable(vaultClient, pair)
		if envname != "" {
			os.Setenv(envname , envvar)
		}
  }
	// Parse arguments and populate env vars from keyvault
	flag.Parse()
	for _, arg := range os.Args {
		envname, envvar := parseKeyVaultVariable(vaultClient, arg)
		if envname != "" {
			os.Setenv(envname , envvar)
		}

	}

	log.Info("<<<<<<< Environment AFTER >>>>>>>>>")
	for _, pair := range os.Environ() {  log.Debug( pair )  }


	if len(os.Args) == 1 {
		log.Fatalf("%s no command is given, currently vault-env can't determine the entrypoint (command), please specify it explicitly", logPrefix)
	} else {
		binary, err := exec.LookPath(os.Args[1])
		if err != nil {
			log.Errorf("%s binary not found: %s", logPrefix, os.Args[1])
		}
		log.Infof("starting process %s %v", binary, os.Args[1:])
		err = syscall.Exec(binary, os.Args[1:], environ)
		if err != nil {
			log.Errorf("%s failed to exec process '%s': %s", logPrefix, binary, err.Error())
		}
	}

	log.Debugf("%s azure key vault env injector successfully injected env variables with secrets", logPrefix)
	log.Debugf("%s shutting down azure key vault env injector", logPrefix)
}

func printEnv() {
	environ := os.Environ()
	for _, pair := range environ {  log.Debug( pair )  }
}

func parseKeyVaultVariable(vaultClient keyvault.BaseClient, arg string) (string, string) {

	if strings.Contains(arg, "=") {
		envsplit := strings.Split( arg, "=" )
		if strings.Contains( envsplit[1], "@") {
			vaultsplit := strings.Split( envsplit[1], "@" )
			log.Infof("parseKeyVaultVariable: parsing vault service for vault '%s', with key '%s'", vaultsplit[1] , vaultsplit[0] )
			if vaultsplit[0] != "" {
				secretResp, err :=  getSecret( vaultClient, vaultsplit[1], vaultsplit[0])
				if err != nil {
					log.Errorf("%s unable to get value for secret:  %s", logPrefix, err.Error())
					return "", ""
				}else{
					log.Info(">>> secretResp.Value: " + *secretResp.Value)
					//os.Setenv(envsplit[0] , *secretResp.Value)
					return envsplit[0] , *secretResp.Value
				}
			}
		}else{
			log.Info("parseKeyVaultVariable: Skipping argument value from parsing: " + arg)
			return "", ""
		}
	}else{
		log.Info("Skipping argument: " + arg)
		return "", ""
	}
	return "", ""

}

func getSecret(vaultClient keyvault.BaseClient, vaultname string, secname string) (result keyvault.SecretBundle, err error) {
	log.Info("Making a call to: " + "https://"+vaultname+".vault.azure.net" + " to retrieve value for key: " + secname)
	return vaultClient.GetSecret( context.Background(), "https://"+vaultname+".vault.azure.net", secname, "")
}

func listSecrets(vaultClient keyvault.BaseClient) {
	secretList, err := vaultClient.GetSecrets(context.Background(), "https://"+vaultName+".vault.azure.net", nil)
	if err != nil {
		fmt.Printf("unable to get list of secrets: %v\n", err)
		os.Exit(1)
	}
	// group by ContentType
	secWithType := make(map[string][]string)
	secWithoutType := make([]string, 1)
	for _, secret := range secretList.Values() {
		if secret.ContentType != nil {
			_, exists := secWithType[*secret.ContentType]
			if exists {
				secWithType[*secret.ContentType] = append(secWithType[*secret.ContentType], path.Base(*secret.ID))
			} else {
				tempSlice := make([]string, 1)
				tempSlice[0] = path.Base(*secret.ID)
				secWithType[*secret.ContentType] = tempSlice
			}
		} else {
			secWithoutType = append(secWithoutType, path.Base(*secret.ID))
		}
	}

	for k, v := range secWithType {
		fmt.Println(k)
		for _, sec := range v {
			fmt.Println(" |--- " + sec)
		}
	}
	for _, wov := range secWithoutType {
		fmt.Println(wov)
	}
}


func logRequest() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r, err := p.Prepare(r)
			if err != nil {
				log.Println(err)
			}
			dump, _ := httputil.DumpRequestOut(r, true)
			log.Println(string(dump))
			return r, err
		})
	}
}

func logResponse() autorest.RespondDecorator {
	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			err := p.Respond(r)
			if err != nil {
				log.Println(err)
			}
			dump, _ := httputil.DumpResponse(r, true)
			log.Println(string(dump))
			return err
		})
	}
}
