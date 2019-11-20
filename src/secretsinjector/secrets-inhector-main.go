
package secretsinjector

import (
	"encoding/json"
	"fmt"
	"strings"
	"errors"

	/*
		"context"
		"errors"
		"io/ioutil"
		"net/http"
		"net/http/httputil"
	*/

	"os"

	//	"os/exec"
	//	"strings"
	//	"syscall"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	//	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	//	"github.com/Azure/go-autorest/autorest"
)
const (
	logPrefix              = "secret-injector:"
	vaultVarName           = "AzureKeyVault"
	patternSecretName      = "secret_injector_secret_name_"
	patternSecretMountPath = "secret_injector_mount_path_"
)
//------------------------------------------------------------------------------

//
// Secret-Vault Env Variable struct
//
type SecretVaultEnvVariableStruct struct {
	SecName    	string `json: "SecName,omitempty"`
	VaultName  	string	`json: "VaultName,omitempty"` // reserved for future development
	EnvVarName 	string `json: "EnvVarName,omitempty"`
	Secret		string `json: "Secret,omitempty"`
	IsValid    	bool `json: "IsValid,omitempty"`
}
//
// Secret-Vault File struct
//
type SecretVaultFileVariableStruct struct {
	SecName    	string `json: "SecName,omitempty"`
	VaultName  	string `json: "VaultName,omitempty"` // reserved for future development
	FileMntPath string `json: "FileMntPath,omitempty"`
	Secret 		string `json: "Secret,omitempty"`
	IsValid    	bool `json: "IsValid,omitempty"`
}
//
// Secrets Injector struct
//
type SecretsInjectorStruct struct {
	VaultNameDefault   	string `json: "VaultNameDefault,omitempty"`
	VaultClient       	keyvault.BaseClient `json:"-"`
	EnvVarSecrets 		[]SecretVaultEnvVariableStruct `json: "EnvVarSecrets,omitempty"`
	FileSecrets 		[]SecretVaultFileVariableStruct `json: "FileSecrets,omitempty"`
}
//------------------------------------------------------------------------------
func (self *SecretsInjectorStruct) MarshalEnvVarToJson() (string, error) {
	jstr, err := json.Marshal( self )
	if err != nil {
		return "", err
	} else {
		return string( jstr ), nil
	}
}
func (self *SecretsInjectorStruct) New() (SecretsInjectorStruct, error) {

	self.EnvVarSecrets = make([]SecretVaultEnvVariableStruct, 0)
	self.FileSecrets = make([]SecretVaultFileVariableStruct, 0)

	for _, pairEnvVar := range os.Environ() {
		fmt.Println("Processing env var: ", pairEnvVar)
		self.setDefaultVault(pairEnvVar)
		_ = self.initEnvVars(pairEnvVar)
		_ = self.initFileVars(pairEnvVar)
	}
	json, _ := self.MarshalEnvVarToJson()
	fmt.Println("fyi... marshaled: ", json)

	return *self, nil
}
//------------------------------------------------------------------------------
func (self *SecretsInjectorStruct) setDefaultVault(pair string) {
	envVarSplit := strings.Split(pair, "=")
	if envVarSplit[0] != "" && strings.TrimSpace(strings.ToLower(envVarSplit[0])) == strings.ToLower(vaultVarName) {  self.VaultNameDefault = envVarSplit[1]  }
}
//------------------------------------------------------------------------------
// Section deals with Env Variables Secrets
func (self *SecretsInjectorStruct) initEnvVars(pair string) error {
	v, err := (&SecretVaultEnvVariableStruct{}).parse(pair)
	if err == nil {
		self.addEnvVar(v)
	}
	return nil
}
func (self *SecretVaultEnvVariableStruct) parse (item string) (SecretVaultEnvVariableStruct, error) {
	envVarSplit := strings.Split(item, "=") ; secNameSplit := strings.Split(envVarSplit[1], "@")
	if len(secNameSplit) != 2 {
		return SecretVaultEnvVariableStruct{}, errors.New("Does not match pattern")
	}else{
		return SecretVaultEnvVariableStruct{
			SecName: secNameSplit[0],
			VaultName: secNameSplit[1],
			EnvVarName: envVarSplit[0],
			Secret: "",
			IsValid: false,
		}, nil
	}
	return SecretVaultEnvVariableStruct{}, nil
}
func (self *SecretsInjectorStruct) addEnvVar (item SecretVaultEnvVariableStruct) []SecretVaultEnvVariableStruct {
	self.EnvVarSecrets = append(self.EnvVarSecrets, item)
	return self.EnvVarSecrets
}
//------------------------------------------------------------------------------
// Section deals with File based Secrets
func (self *SecretsInjectorStruct) initFileVars(pair string) error {
	v, err := (&SecretVaultFileVariableStruct{}).parse(pair)
	if err == nil {
		self.addFileVar(v)
	}
	return nil
}
func (self *SecretsInjectorStruct) addFileVar (item SecretVaultFileVariableStruct) []SecretVaultFileVariableStruct {
	self.FileSecrets = append(self.FileSecrets, item)
	return self.FileSecrets
}
func (self *SecretVaultFileVariableStruct) parse (item string) (SecretVaultFileVariableStruct, error) {
	envVarSplit := strings.Split(item, "=") ; envSecName := envVarSplit[1]
	// matching to pattern
	if envVarSplit[0] != "" && strings.Contains(strings.TrimSpace(strings.ToLower(envVarSplit[0])) , strings.ToLower(patternSecretName)) {
		envSecSubName := stringBetween( strings.ToLower(item), strings.ToLower( patternSecretName ), "=" )
		mntPath := getEnvVariableByName( strings.ToLower( patternSecretMountPath+ envSecSubName ) )
		// populate SecretVaultFileVariableStruct
		return SecretVaultFileVariableStruct{
			SecName: envSecName,
			VaultName: "",
			FileMntPath: mntPath,
			Secret: "",
			IsValid: false,
		}, nil
	}
	s := fmt.Sprintf("Could not parse variable: %s ",item )
	return SecretVaultFileVariableStruct{}, errors.New(s)
}

//------------------------------------------------------------------------------
// Utils
//
// Utility function designed to extract substring between 2 strings
//
func stringBetween(value string, a string, b string) string {
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
// Function retrieves environment variable value based on its name
//
func getEnvVariableByName(variableName string) string {
	environ := os.Environ()
	for _, pair := range environ {
		if strings.Contains(pair, "=") {
			if split := strings.Split(pair, "="); strings.EqualFold(strings.TrimSpace(variableName), strings.TrimSpace(split[0])) { return split[1] }
		}
	}
	return ""
}


