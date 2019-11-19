
package secretsinjector

import (
	"context"
	"errors"
	"fmt"
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
)

var (
	injector SecretsInjectorStruct
)

const (
	logPrefix              = "secret-injector:"
	vaultVarName           = "AzureKeyVault"
	patternSecretName      = "secret_injector_secret_name_"
	patternSecretMountPath = "secret_injector_mount_path_"
)

//
// Secret-Vault Env Variable struct
//
type SecretVaultEnvVariableStruct struct {
	secName    	string
	vaultName  	string	// reserved for future development
	envVarName 	string
	secret		string
	isValid    	bool
}
//
// Secret-Vault File struct
//
type SecretVaultFileVariableStruct struct {
	secName    	string
	vaultName  	string // reserved for future development
	fileMntPath string
	secret 		string
	isValid    	bool
}
//
// Secrets Injector struct
//
type SecretsInjectorStruct struct {
	vaultNameDefault   	string
	//vaultVariableName	string
	vaultClient       	keyvault.BaseClient
	envVarSecrets 		[]SecretVaultEnvVariableStruct
	fileSecrets 		[]SecretVaultFileVariableStruct
}

//------------------------------------------------------------------------------
func (self *SecretsInjectorStruct) New() (SecretsInjectorStruct, error) {
	self.envVarSecrets = []SecretVaultEnvVariableStruct{}
	self.fileSecrets = []SecretVaultFileVariableStruct{}

	for _, pairEnvVar := range os.Environ() {
		_ := self.initEnvVars(pairEnvVar)
		_ := self.initFileVars(pairEnvVar)

	}

	return *self, nil
}

func (self *SecretsInjectorStruct) initEnvVars(pair string) error {
	return nil
}
func (self *SecretsInjectorStruct) addEnvVar (item SecretVaultEnvVariableStruct) []SecretVaultEnvVariableStruct {
	self.envVarSecrets = append(self.envVarSecrets, item)
	return self.envVarSecrets
}
func (self *SecretVaultEnvVariableStruct) parse (item string) (SecretVaultEnvVariableStruct, error) {
	return SecretVaultEnvVariableStruct{}, nil
}


func (self *SecretsInjectorStruct) initFileVars(pair string) error {
	return nil
}
func (self *SecretsInjectorStruct) addFileVar (item SecretVaultFileVariableStruct) []SecretVaultFileVariableStruct {
	self.fileSecrets = append(self.fileSecrets, item)
	return self.fileSecrets
}
func (self *SecretVaultFileVariableStruct) parse (item string) (SecretVaultFileVariableStruct, error) {
	return SecretVaultFileVariableStruct{}, nil
}

//------------------------------------------------------------------------------


