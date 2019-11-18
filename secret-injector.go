package main

import (
	"context"
	"errors"
	"fmt"
	_ "github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/sirupsen/logrus"
)

//------------------------------------------------------------------------------
// Secrets Injector struct
type azureSecretsInjector struct {
	vaultName         string
	vaultVariableName string
	vaultClient       keyvault.BaseClient
}

//
// Getter of existing azureSecretsInjector instance
//
func (injector azureSecretsInjector) Get() azureSecretsInjector {
	return injector
}

//
// Utility function to set baseClient
//
func (injector azureSecretsInjector) SetVaultClient(vc keyvault.BaseClient) {
	injector.vaultClient = vc
}

//
// Function generates new vault authorizer
//
func (injector azureSecretsInjector) NewKeyvaultClient() keyvault.BaseClient {
	var bc keyvault.BaseClient
	authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		Errorf("Can't initialize authorizer: %v\n", err)
		return bc
	}
	bc = keyvault.New()
	bc.Authorizer = authorizer
	return bc
}

//
// Function creates new instance of azureSecretsInjector
//
func (injector azureSecretsInjector) New() azureSecretsInjector {
	return azureSecretsInjector{
		vaultVariableName: vaultVarName,
		vaultName:         getEnvVariableByName(vaultVarName),
		vaultClient:       injector.NewKeyvaultClient(),
	}
}

//
// Function to retrive the secret from the vault based on its name
//
func (injector azureSecretsInjector) retrieveSecret(secName string) (string, error) {
	sec := ""
	secretResp, err := getSecret(injector.vaultClient, injector.vaultName, secName)
	if err != nil {
		s := fmt.Sprintf("%v", err.Error())
		return "", errors.New(s)
	} else {
		sec = *secretResp.Value
	}
	return sec, nil
}

//
// function takes env variable in form "name=value" and looks for pattern (patternSecretName) to match
// if matches, extracts the name of the secret
// than, finds matching mount path for given secret name
// return value: (mount path variable name , actual mount path, secret name)
//
func (injector azureSecretsInjector) retrieveSecretMountPath(variable string) (string, string) {

	if strings.Contains( strings.ToLower(variable), patternSecretName) {
		var secName string
		Debugf("Found matching env variable: %s", strings.ToLower(variable))

		secSeq := between( strings.ToLower(variable), patternSecretName, "=")
		split := strings.Split( strings.ToLower(variable), "=" )
		if split[1] != "" {
			secName = split[1]
		}

		var mntPath = getEnvVariableByName( patternSecretMountPath + secSeq )
		if mntPath != "" {
			Debugf("Found matching mount path '%s' for secret '%s'", mntPath, secName)
			return mntPath, secName
		} else {
			Debugf("Can't find matching mount path for secret '%s'", secName)
		}
		Debugf("Injecting file-based secret : %s to file %s ", secName, mntPath)

	} else {
		Debugf("Skipping variable: %s", strings.ToLower(variable))
	}
	return "", ""
}

//
// function takes secret-name@AzureKeyVault and returns (secret-name, actual secret from vault)
//
func (injector azureSecretsInjector) parseEnvKeyVaultVariable(arg string) (string, string) {

	var secName string

	if strings.Contains(arg, "=") && strings.Contains(arg, "@") {
		envVarSplit := strings.Split(arg, "=")

		if strings.Contains(envVarSplit[1], "@") { // detected explicate assigment of vault
			tmp := strings.Split( string(envVarSplit[1]), "@" )
			secName = tmp[0]
			Infof("Explicit Vault assigment detected : %s", arg)
		} else {
			Infof("Warning: Implicit Vault assigment detected : %s", arg)
			secName = envVarSplit[1]
		}

		Debugf("engaging vault service for vault '%s', with key '%s'", injector.vaultName, secName )
		if secName != "" {
				secretResp, err := getSecret(injector.vaultClient, injector.vaultName, secName )
				if err != nil {
					Errorf("%s unable to get value for secret %s from vault %s. Error:  %v", logPrefix, secName, injector.vaultName, err.Error() )
					return "", ""
				} else {
					Debugf(">>> secretResp.Value: %s", *secretResp.Value)
					return envVarSplit[0], *secretResp.Value
				}
		}
		Infof("Parsing argument value from env: %s", arg)

	} else {
		Info("Skipping argument: " + arg)
		return "", ""
	}
	return "", ""
}

//------------------------------------------------------------------------------
var (
	injector azureSecretsInjector
)

const (
	logPrefix              = "secret-injector:"
	vaultVarName           = "AzureKeyVault"
	patternSecretName      = "secret_injector_secret_name_"
	patternSecretMountPath = "secret_injector_mount_path_"
)

//------------------------------------------------------------------------------
//
// initialize/set environment
//
func init() {
	SetFormatter(&TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	// setting debug mode
	debug := getEnvVariableByName("debug")
	if strings.EqualFold( debug, "true" ) {
		SetLevel(DebugLevel)
	} else {
		SetLevel(InfoLevel)
	}
	SetFormatter(&TextFormatter{})
	// custom auth
	_ = os.Setenv("CUSTOM_AUTH_INJECT", "true")
	// populate azureSecretsInjector struct
	injector = injector.New()

	Debugf("vaultName: %s\n", injector.vaultName)
}
//
// main function
//
func main() {
	Infof("%s Starting azure key vault env injector", logPrefix)
	Debug("<<<<<<< Environment BEFORE >>>>>>>>>")
	printEnv()

	for _, pair := range os.Environ() {
		//
		// Parse env variables and populate env vars from keyvault
		//
		envname, envvar := injector.parseEnvKeyVaultVariable(pair)
		if envname != "" {
			_ = os.Setenv(envname, envvar)
		}
		//
		// Parse env variables and populate files with secrets
		//
		mntPath, secName := injector.retrieveSecretMountPath(pair)
		if mntPath != "" {

			// get secret based on the name
			secret, err := injector.retrieveSecret(secName)
			if err != nil {
				Errorf("%s unable to get value for secret:  %v", logPrefix, err.Error())
			}
			err = generateSecretsFile(mntPath, secName, secret)
			if err != nil {
				Errorf("%s unable to generate secrets file:  %v", logPrefix, err.Error())
			}
		}
	}

	Debug("<<<<<<< Environment AFTER >>>>>>>>>")
	printEnv()

	if len(os.Args) == 1 {
		Fatalf("%s no command is given, currently vault-env can't determine the entrypoint (command), please specify it explicitly", logPrefix)
	} else {
		binary, err := exec.LookPath(os.Args[1])
		if err != nil {
			Errorf("%s binary not found: %s", logPrefix, os.Args[1])
		}
		Infof("starting process %s %v", binary, os.Args[1:])
		err = syscall.Exec( binary, os.Args[1:], os.Environ() )
		if err != nil {
			Errorf("%s failed to exec process '%s': %s", logPrefix, binary, err.Error())
		}
	}

	Debugf("%s azure key vault env injector successfully injected env variables with secrets", logPrefix)
	Debugf("%s shutting down azure key vault env injector", logPrefix)
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
// Prints all environment values
//
func printEnv() {
	environ := os.Environ()
	for _, pair := range environ {
		Debug(pair)
	}
}

//
// Function retrieves environment variable value based on its name
//
func getEnvVariableByName(variableName string) string {
	environ := os.Environ()
	for _, pair := range environ {
		Debug(pair)
		if strings.Contains(pair, "=") {
			split := strings.Split(pair, "=")
			if strings.EqualFold(strings.TrimSpace(variableName), strings.TrimSpace(split[0])) {
				return split[1]
			}
		}
	}
	return ""
}

//
// Function  creates secrets file, writes secret to it and makes file read-only
//
func generateSecretsFile(mntPath, secName, secret string) error {
	secretsFile := mntPath + "/" + secName
	_, err := os.Create(secretsFile)
	if err != nil {
		s := fmt.Sprintf("Error creating the file %s: %v", secretsFile, err.Error())
		return errors.New(s)
	} else {
		Debugf("Creating secret file: %s", secretsFile)
		_, err := os.Stat(secretsFile)
		if err != nil {
			if os.IsNotExist(err) {
				s := fmt.Sprintf("File %s does not exist.", secretsFile)
				return errors.New(s)
			}
		} else {
			// write secret to secrtets file
			Debugf("Populating secrets file: %s", secretsFile)
			err := ioutil.WriteFile(secretsFile, []byte( secret ), 0666)
			if err != nil {
				s := fmt.Sprintf("Can't write to the file: %v", err.Error())
				return errors.New(s)
			}
		}
		//make file read-only
		Debugf("Making secrets file: %s read-only", secretsFile)
		err = os.Chmod(secretsFile, 0444)
		if err != nil {
			s := fmt.Sprintf("Can't file's permission mask: %v", err.Error())
			return errors.New(s)
		}

	}
	return nil
}

//
// Low level function to get the secret from the vault based on its name
//
func getSecret(vaultClient keyvault.BaseClient, vaultname string, secname string) (result keyvault.SecretBundle, err error) {
	Debugf("%s Making a call to:  https://%s.vault.azure.net to retrieve value for KEY: %s\n", logPrefix, vaultname, secname)
	return vaultClient.GetSecret(context.Background(), "https://"+vaultname+".vault.azure.net", secname, "")
}

//
// debug function
//
func logRequest() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r, err := p.Prepare(r)
			if err != nil {
				Debugln(err)
			}
			dump, _ := httputil.DumpRequestOut(r, true)
			Debugln(string(dump))
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
				Debugln(err)
			}
			dump, _ := httputil.DumpResponse(r, true)
			Debugln(string(dump))
			return err
		})
	}
}
