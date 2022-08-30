package tpu_quic

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strconv"
	"strings"
)

func CheckIfDuplicate(array []string, item string) bool {
	for _, value := range array {
		if value == item {
			return true
		}
	}
	return false
}

func ConvertURLToWS(url string) string {
	urlWithReplacedProtocol := strings.Replace(strings.Replace(url, "https", "wss", -1), "http", "ws", -1)
	protocolSplit := strings.Split(urlWithReplacedProtocol, "://")
	if strings.Contains(protocolSplit[1], ":") {
		var port string = ""
		portSplit := strings.Split(protocolSplit[1], ":")
		if len(portSplit) == 2 {
			portInt, err := strconv.Atoi(strings.Replace(portSplit[1], "/", "", -1))
			if err != nil {
				return ""
			}
			port = strconv.Itoa(portInt + 1)
			return protocolSplit[0] + "://" + portSplit[0] + ":" + port
		} else {
			return ""
		}
	} else {
		return urlWithReplacedProtocol
	}
}

func NewSelfSignedTLSCertificate(rawIP string, pub ed25519.PublicKey, priv ed25519.PrivateKey) (tls.Certificate, error) {
	eidIdentifier := []int{1, 3, 101, 112}
	template := x509.Certificate{
		SerialNumber: big.NewInt(69),
		Subject: pkix.Name{
			CommonName: "Solana node",
			ExtraNames: []pkix.AttributeTypeAndValue{{eidIdentifier, rawIP}},
		},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	convertedKey, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "ED25519 PRIVATE KEY", Bytes: convertedKey})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	return certificate, nil
}

func NewSelfSignedTLSCertificateChain(rawIP string, pub ed25519.PublicKey, priv ed25519.PrivateKey) (*x509.CertPool, error) {
	eidIdentifier := []int{1, 3, 101, 112}
	template := x509.Certificate{
		SerialNumber: big.NewInt(69),
		Subject: pkix.Name{
			CommonName: "Solana node",
			ExtraNames: []pkix.AttributeTypeAndValue{{eidIdentifier, rawIP}},
		},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	//convertedKey, err := x509.MarshalPKCS8PrivateKey(priv)
	//if err != nil {
	//	return &x509.CertPool{}, err
	//}
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
	if err != nil {
		return &x509.CertPool{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	return pool, nil
}
