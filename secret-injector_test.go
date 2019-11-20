package main

import (
	"os"
	"testing"
	)

func TestKeyVaultName(t *testing.T) {
	t.Log("Testing name of key vault")
	os.Setenv("AzureKeyVault", "test-vault")
	if getEnvVariableByName("AzureKeyVault") == "" {
		t.Errorf("environment varibale AzureKeyVault expected to be set")
	}
}
func TestBetweenUtility(t *testing.T) {
	t.Log("Testing between utility function")
	if between("SECRET_INJECTOR_SECRET_NAME_secret2=secret1", "SECRET_INJECTOR_SECRET_NAME_", "=") != "secret2" {
		t.Errorf("Function 'between' suppose to return the valid string")
	}
}
func TestRetrieveSecretMountPath(t *testing.T) {
	t.Log("Testing RetrieveSecretMountPath function")
	os.Setenv("secret_injector_mount_path_1", "/etc/secrets")
	pair := "secret_injector_secret_name_1=secret-1"
	mntPath, secName := injector.retrieveSecretMountPath(pair)
	if mntPath != "/etc/secrets" {
		t.Errorf("Function retrieveSecretMountPath did not produce correct mounting path: %s", mntPath)
	}
	if secName != "secret-1" {
		t.Errorf("Function retrieveSecretMountPath did not produce correct secret name: %s", secName)
	}
}

func TestDetectSecretVaultPattern(t *testing.T) {
	t.Log("Testing detectSecretVaultPattern function")
	if detectSecretVaultPattern("envVar=secret-name@AzureKeyVault") != true {
		t.Errorf("Function detectSecretVaultPattern did not detect correct pattern" )
	}
	if detectSecretVaultPattern("envVar=secret-name@") != true {
		t.Errorf("Function detectSecretVaultPattern did not detect correct pattern" )
	}
}



