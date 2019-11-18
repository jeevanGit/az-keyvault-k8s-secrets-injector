package main

import (
	"testing"
	)

func TestKeyVaultName(t *testing.T) {
	t.Log("Testing name of key vault")
	if getEnvVariableByName("AzureKeyVault") == "" {
		t.Errorf("environment varibale AzureKeyVault expected to be set")
	}
}
func TestBetweenUtility(t *testing.T) {
	t.Log("Testing between utility function")
	if between("SECRET_INJECTOR_SECRET_NAME_secret2=secret1", "SECRET_INJECTOR_SECRET_NAME_", "=") != "secret2" {
		t.Errorf("environment varibale AzureKeyVault expected to be set")
	}
}
