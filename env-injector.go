package main

import (
	_ "encoding/json"
	_ "fmt"
	_ "github.com/spf13/viper"
	"context"
	_ "flag"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"syscall"
	"os/exec"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest"
	_ "k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/rest"

	log "github.com/sirupsen/logrus"
)

type azureEnvInjectorConfig struct {
	vaultName 				string
	vaultVariableName	string
}
var (
	serviceAppName    string
	config azureEnvInjectorConfig
)
const (
	logPrefix    = "env-injector:"
	vaultVarName = "AzureKeyVault"
)
//------------------------------------------------------------------------------

func init() {
	log.SetFormatter( &log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	if strings.EqualFold( os.Getenv("debug"), "true" ){
		log.SetLevel(log.DebugLevel)
	}else{
		log.SetLevel(log.InfoLevel)
	}

/*
	viper.SetConfigName("config")
	viper.AddConfigPath(".") ; viper.AddConfigPath("/")
	err := viper.ReadInConfig(); if err != nil {
		panic( fmt.Errorf("Fatal error config file: %s \n", err) )
	}
*/
	// init from confif file
	//vaultVariableName = viper.GetString("service.vault.vaultVariableName")
	// get name of the vault
	//vaultName = getEnvVariableByName(vaultVariableName)
	//if vaultName == "" {
	//	log.Errorf("%s Unable to retrive Vault's name.", logPrefix)
	//}
	config = azureEnvInjectorConfig{
		vaultName: vaultVarName,
		vaultVariableName: "",
	}
	log.Infof("vaultVariableName: %s\n", config.vaultVariableName)
	log.Infof("vaultName: %s\n", config.vaultName)
}

func main() {
	log.Debugf("%s Starting azure key vault env injector", logPrefix)
	os.Setenv("CUSTOM_AUTH_INJECT", "true")
	authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Errorf("unable to create vault authorizer: %v\n", err)
	}else{
		log.Info("Starting authorizer...")
	}
	vaultClient := keyvault.New()
	vaultClient.Authorizer = authorizer

	log.Debug("<<<<<<< Environment BEFORE >>>>>>>>>")
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
	/*
	flag.Parse()
	for _, arg := range os.Args {
		envname, envvar := parseKeyVaultVariable(vaultClient, arg)
		if envname != "" {
			os.Setenv(envname , envvar)
		}

	}
	*/

	log.Debug("<<<<<<< Environment AFTER >>>>>>>>>")
	for _, pair := range os.Environ() {  log.Info( pair )  }

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

func getEnvVariableByName(variableName string) (string) {
	environ := os.Environ()
	for _, pair := range environ {  log.Debug( pair )
		if strings.Contains(pair, "=") {
			split := strings.Split( pair, "=" )
			if strings.EqualFold( strings.TrimSpace(variableName), strings.TrimSpace(split[0]) ) {
				return split[1]
			}
		}
	}
	return ""
}

// function takes secret-name@AzureKeyVault and returns (secret-name, actual secret from vault)
func parseKeyVaultVariable(vaultClient keyvault.BaseClient, arg string) (string, string) {

	if strings.Contains(arg, "=") {
		envsplit := strings.Split( arg, "=" )
		if strings.Contains( envsplit[1], "@") {
			vaultsplit := strings.Split( envsplit[1], "@" )

			vname := getEnvVariableByName(config.vaultName)

			log.Debugf("parseKeyVaultVariable: parsing vault service for vault '%s', with key '%s'", vname, vaultsplit[0] )
			if vaultsplit[0] != "" {
				secretResp, err :=  getSecret( vaultClient, vname, vaultsplit[0] )

				if err != nil {
					log.Errorf("%s unable to get value for secret:  %v", logPrefix, err.Error())
					return "", ""
				}else{
					log.Debugf(">>> secretResp.Value: %s", *secretResp.Value)
					return envsplit[0] , *secretResp.Value
				}
			}
			log.Infof("parseKeyVaultVariable: Parsing argument value from env: %s", arg)
		}else{
			log.Infof("parseKeyVaultVariable: Skipping argument value from env: %s", arg)
			return "", ""
		}
	}else{
		log.Info("Skipping argument: " + arg)
		return "", ""
	}
	return "", ""
}

func getSecret(vaultClient keyvault.BaseClient, vaultname string, secname string) (result keyvault.SecretBundle, err error) {
	log.Debugf("%s Making a call to:  https://%s.vault.azure.net to retrieve value for KEY: %s\n", logPrefix, vaultname, secname)
	return vaultClient.GetSecret( context.Background(), "https://"+vaultname+".vault.azure.net", secname, "")
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
