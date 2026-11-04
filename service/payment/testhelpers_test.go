package payment

import (
	"crypto/rsa"
	"crypto/x509"
	"testing"
)

func x509MarshalPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal pkcs8: %v", err)
	}
	return der
}

func x509MarshalPKIX(t *testing.T, key *rsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("failed to marshal pkix: %v", err)
	}
	return der
}
