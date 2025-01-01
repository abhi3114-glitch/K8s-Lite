package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	// 1. Generate CA
	log.Println("Generating CA...")
	caPriv, caCertBytes := generateCert("K8s-Lite-CA", nil, nil, true)
	save("ca", caPriv, caCertBytes)

	caCert, _ := x509.ParseCertificate(caCertBytes)

	// 2. Generate API Server Cert
	log.Println("Generating Server Cert...")
	// IP SANs: localhost, 127.0.0.1
	serverPriv, serverCertBytes := generateCert("k8s-lite-apiserver", caCert, caPriv, false, "127.0.0.1", "localhost")
	save("server", serverPriv, serverCertBytes)

	// 3. Generate Admin Client Cert
	log.Println("Generating Admin Client Cert...")
	adminPriv, adminCertBytes := generateCert("admin", caCert, caPriv, false)
	save("client-admin", adminPriv, adminCertBytes)

	// 4. Generate Kubelet Client Cert
	log.Println("Generating Kubelet Client Cert...")
	nodePriv, nodeCertBytes := generateCert("kubelet", caCert, caPriv, false)
	save("client-kubelet", nodePriv, nodeCertBytes)

	// 5. Generate Controller Manager Client Cert
	log.Println("Generating Controller Manager Client Cert...")
	cmPriv, cmCertBytes := generateCert("controller-manager", caCert, caPriv, false)
	save("client-cm", cmPriv, cmCertBytes)

	log.Println("Done! Certificates generated.")
}

func generateCert(cn string, parentCert *x509.Certificate, parentKey *rsa.PrivateKey, isCA bool, sans ...string) (*rsa.PrivateKey, []byte) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			Organization: []string{"K8s-Lite"},
			CommonName:   cn,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour), // 1 Year

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	for _, s := range sans {
		if ip := net.ParseIP(s); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, s)
		}
	}

	// Self-sign if no parent
	parent := template
	signKey := priv
	if parentCert != nil {
		parent = parentCert
		signKey = parentKey
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &priv.PublicKey, signKey)
	if err != nil {
		log.Fatal(err)
	}

	return priv, certBytes
}

func save(name string, priv *rsa.PrivateKey, certBytes []byte) {
	// Save Key
	keyOut, _ := os.Create(name + ".key")
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()

	// Save Cert
	certOut, _ := os.Create(name + ".pem")
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	certOut.Close()
}









