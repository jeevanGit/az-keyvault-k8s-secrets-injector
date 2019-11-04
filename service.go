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

	log "github.com/sirupsen/logrus"
)

var (
	servicePort       string
	serviceApiVersion string
	serviceAppName    string
	// legacy
	serviceDebugFlag  bool
	configFormat	  string

	vaultName string
)

type GitCredentials struct {
		RepoName string `json:"repo_name"`
		Account  string `json:"account"`
		ApiToken string `json:"api_token"`
}

const (
	logPrefix    = "env-injector:"
	envLookupKey = "@azurekeyvault"
)

func init() {
	//args := os.Args

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	//log.SetReportCaller(true)
	viper.SetConfigName("config")
	viper.AddConfigPath(".") ; viper.AddConfigPath("/")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	serviceAppName = viper.GetString("service.name")

}

func main() {
	log.Info("Starting service '" + serviceAppName + "'...")

	os.Setenv("CUSTOM_AUTH_INJECT", "true")

	os.Setenv("AZURE_TENANT_ID", viper.GetString("creds.AZURE_TENANT_ID"))
	os.Setenv("AZURE_CLIENT_ID", viper.GetString("creds.AZURE_CLIENT_ID"))
	os.Setenv("AZURE_CLIENT_SECRET", viper.GetString("creds.AZURE_CLIENT_SECRET"))

	authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Errorf("unable to create vault authorizer: %v\n", err)
	}else{
		log.Info("Starting authorizer...")
	}
	basicClient := keyvault.New()
	basicClient.Authorizer = authorizer
	//basicClient.RequestInspector = logRequest()
	//basicClient.ResponseInspector = logResponse()

	flag.Parse()
	for _, arg := range os.Args {

		if strings.Contains(arg, "=") {
			envsplit := strings.Split( arg, "=" )
			if strings.Contains( envsplit[1], "@") {
				vaultsplit := strings.Split( envsplit[1], "@" )
				log.Infof("INFO: Starting vault service for vault '%s', with key '%s'", vaultsplit[1] , vaultsplit[0] )
				//fmt.Printf("vault name: %s and it's key %s \n", vaultsplit[1], vaultsplit[0])
				if vaultsplit[0] != "" {
					secretResp, err :=  getSecret( basicClient, vaultsplit[1], vaultsplit[0])
					if err != nil {
						//fmt.Printf("unable to get value for secret: %v\n", err)
						log.Errorf("%s unable to get value for secret:  %s", logPrefix, err.Error())
					}else{
						log.Info(">>> secretResp.Value: " + *secretResp.Value)
						os.Setenv(envsplit[0] , *secretResp.Value)
					}
				}
			}else{
				log.Info("Skipping argument value from parsing: " + arg)
			}
		}else{
			log.Info("Skipping argument: " + arg)
		}

	}

	environ := os.Environ()
	for _, pair := range environ {
    log.Info( pair )
  }

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
	log.Debugf("%s azure key vault env injector", logPrefix)

}

func getSecret(basicClient keyvault.BaseClient, vaultname string, secname string) (result keyvault.SecretBundle, err error) {
	log.Info("Making a call to: " + "https://"+vaultname+".vault.azure.net" + " to retrieve value for key: " + secname)
	return basicClient.GetSecret(context.Background(), "https://"+vaultname+".vault.azure.net", secname, "")
}

func listSecrets(basicClient keyvault.BaseClient) {
	secretList, err := basicClient.GetSecrets(context.Background(), "https://"+vaultName+".vault.azure.net", nil)
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
