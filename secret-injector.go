package main

import (
	_ "encoding/json"
	"fmt"
	_ "github.com/spf13/viper"
	"context"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"syscall"
	"os/exec"
	"io/ioutil"
	"errors"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest"

	log "github.com/sirupsen/logrus"
	_ "github.com/x-cray/logrus-prefixed-formatter"
)

//------------------------------------------------------------------------------
type azureSecretsInjector struct {
	vaultName 				string
	vaultVariableName	string
	vaultClient keyvault.BaseClient
}

func (injector azureSecretsInjector) Get() azureSecretsInjector{
	return injector
}

func (injector azureSecretsInjector) SetVaultClient(vc keyvault.BaseClient){
	injector.vaultClient = vc
}

func (injector azureSecretsInjector) NewKeyvaultClient() keyvault.BaseClient{
	var bc keyvault.BaseClient
	authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Errorf("Can't initialize authorizer: %v\n", err)
		return bc
	}
	bc = keyvault.New()
	bc.Authorizer = authorizer
	return bc
}
func (injector azureSecretsInjector) New() azureSecretsInjector{
	return azureSecretsInjector{
		vaultVariableName: vaultVarName,
		vaultName: getEnvVariableByName(vaultVarName),
		vaultClient: injector.NewKeyvaultClient(),
	}
}

//------------------------------------------------------------------------------

var (
	serviceAppName    string
	injector azureSecretsInjector
)
const (
	logPrefix    = "secret-injector:"
	vaultVarName = "AzureKeyVault"
	patternSecretName = "secret_injector_secret_name_"
	patternSecretMountPath = "secret_injector_mount_path_"
)
//------------------------------------------------------------------------------

func init() {
	log.SetFormatter( &log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	// setting debug mode
	if strings.EqualFold( os.Getenv("debug"), "true" ){
		log.SetLevel(log.DebugLevel)
	}else{
		log.SetLevel(log.InfoLevel)
	}
	log.SetFormatter(&log.TextFormatter{})
	// custom auth
	os.Setenv("CUSTOM_AUTH_INJECT", "true")
	// populate azureSecretsInjector struct
  injector = injector.New()

	log.Debugf("vaultName: %s\n", injector.vaultName )
}

func main() {
	log.Infof("%s Starting azure key vault env injector", logPrefix)
	log.Debug("<<<<<<< Environment BEFORE >>>>>>>>>")
		printEnv()

	environ := os.Environ()
	for _, pair := range environ {
		//
		// Parse env variables and populate env vars from keyvault
		//
		envname, envvar := parseEnvKeyVaultVariable(injector.vaultClient, pair)
		if envname != "" {
			os.Setenv(envname , envvar)
		}
		//
		// Parse env variables and populate files with secrets
		//
		mntPath, secName := retrieveSecretMountPath( pair )
		if mntPath != "" {

			// get secret based on the name
			secret, err := retrieveSecret(secName)
			if err != nil {
				log.Errorf("%s unable to get value for secret:  %v", logPrefix, err.Error())
			}

			err = generateSecretsFile( mntPath, secName, secret )
			if err != nil {
				log.Errorf("%s unable to generate secrets file:  %v", logPrefix, err.Error())
			}
		}
  }

	log.Debug("<<<<<<< Environment AFTER >>>>>>>>>")
		printEnv()

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
//
// Utility function designed to extract substring between 2 strings
//
func between(value string, a string, b string) string {
    // Get substring between two strings.
    posFirst := strings.Index(value, a)
    if posFirst == -1 {
        return ""
    }
    posLast := strings.Index(value, b)
    if posLast == -1 {
        return ""
    }
    posFirstAdjusted := posFirst + len(a)
    if posFirstAdjusted >= posLast {
        return ""
    }
    return value[posFirstAdjusted:posLast]
}
//
// Pronts all environment values
//
func printEnv() {
	environ := os.Environ()
	for _, pair := range environ {  log.Debug( pair )  }
}
//
// Function retrieves environment variable value based on its name
//
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
//
// Function to retrive the secret from the vault based on its name
//
func retrieveSecret(secName) (string, error){
	sec := ""
	secretResp, err :=  getSecret( injector.vaultClient, injector.vaultName, secName )
	if err != nil {
		s := fmt.Sprintf("%v", err.Error())
		return "", errors.New(s)
	}else{
		sec = *secretResp.Value
	}
	return sec, nil
}
//
// Function  creates secrets file, writes secret to it and makes file read-only
//
func generateSecretsFile(mntPath, secName, secret string) (error) {
	secretsFile := mntPath + "/" + secName
	_, err := os.Create( secretsFile )
	if err != nil {
			//log.Error( err )
			s := fmt.Sprintf("Error creating the file %s: %v", secretsFile, err.Error())
			return errors.New(s)
	}else {
		log.Debugf("Creating secret file: %s", secretsFile )
		_, err := os.Stat(secretsFile)
		if err != nil {
				if os.IsNotExist(err) {
						//log.Errorf("File %s does not exist.", secretsFile)
						s := fmt.Sprintf("File %s does not exist.", secretsFile)
						return errors.New(s)
				}
		}else {
			// write secret to secrtets file
			log.Debugf("Populating secrets file: %s", secretsFile )
			err := ioutil.WriteFile(secretsFile, []byte( secret ), 0666)
			if err != nil {
					//log.Errorf("Can't write to the file: %v", err.Error() )
					s := fmt.Sprintf("Can't write to the file: %v", err.Error() )
					return errors.New(s)
			}
		}
		//make file read-only
		log.Debugf("Making secrets file: %s read-only", secretsFile )
		err = os.Chmod(secretsFile, 0444)
		if err != nil {
				//log.Errorf("Can't file's permission mask: %v", err.Error() )
				s := fmt.Sprintf("Can't file's permission mask: %v", err.Error() )
				return errors.New(s)
		}

	}
	return nil
}
//
// function takes env variable in form "name=value" and looks for pattern (patternSecretName) to match
// if matches, extracts the name of the secret
// than, finds matching mount path for given secret name
// return value: (mount path varible name , actual mount path, secret name)
//
func retrieveSecretMountPath(variable string) (string, string) {

	if strings.Contains( strings.ToLower(variable) , patternSecretName ) {

		log.Debugf("retrieveSecretMountPath: Found matching env variable: %s", strings.ToLower(variable))
		sec_name := between(strings.ToLower(variable), patternSecretName, "=")
		log.Debugf("retrieveSecretMountPath: secret name: %s", sec_name)

		mnt_path := getEnvVariableByName( patternSecretMountPath + sec_name )
		if mnt_path != "" {
			log.Debugf("retrieveSecretMountPath: Found matching mount path '%s' for secret '%s'", mnt_path, sec_name)
			return mnt_path, sec_name

		}else{
			log.Debugf("retrieveSecretMountPath: Can't find matching mount path for secret '%s'", sec_name)
		}

	}else{
		log.Debugf("retrieveSecretMountPath: Skipping variable: %s", strings.ToLower(variable) )
	}
	return "", ""
}
//
// function takes secret-name@AzureKeyVault and returns (secret-name, actual secret from vault)
//
func parseEnvKeyVaultVariable(vaultClient keyvault.BaseClient, arg string) (string, string) {

	if strings.Contains(arg, "=") {
		envsplit := strings.Split( arg, "=" )
		if strings.Contains( envsplit[1], "@") {
			vaultsplit := strings.Split( envsplit[1], "@" )

			log.Debugf("parseEnvKeyVaultVariable: parsing vault service for vault '%s', with key '%s'", injector.vaultName, vaultsplit[0] )
			if vaultsplit[0] != "" {
				secretResp, err :=  getSecret( vaultClient, injector.vaultName, vaultsplit[0] )

				if err != nil {
					log.Errorf("%s unable to get value for secret:  %v", logPrefix, err.Error())
					return "", ""
				}else{
					log.Debugf(">>> secretResp.Value: %s", *secretResp.Value)
					return envsplit[0] , *secretResp.Value
				}
			}
			log.Infof("parseEnvKeyVaultVariable: Parsing argument value from env: %s", arg)
		}else{
			log.Infof("parseEnvKeyVaultVariable: Skipping argument value from env: %s", arg)
			return "", ""
		}
	}else{
		log.Info("Skipping argument: " + arg)
		return "", ""
	}
	return "", ""
}
//
// Low level function to get the secret from the vault based on its name
//
func getSecret(vaultClient keyvault.BaseClient, vaultname string, secname string) (result keyvault.SecretBundle, err error) {
	log.Debugf("%s Making a call to:  https://%s.vault.azure.net to retrieve value for KEY: %s\n", logPrefix, vaultname, secname)
	return vaultClient.GetSecret( context.Background(), "https://"+vaultname+".vault.azure.net", secname, "")
}
//
// debug function
//
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
//
// debug function
//
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
