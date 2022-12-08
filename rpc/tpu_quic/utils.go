package tpu_quic

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
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

func NewSelfSignedTLSCertificate(ip net.IP) (*x509.CertPool, tls.Certificate, error) {
	//eidIdentifier := []int{1, 3, 101, 112}
	orgPubkey, orgPrivkey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	pk, err := x509.MarshalPKCS8PrivateKey(orgPrivkey)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName: "Solana node",
			//ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 29, 17}, Value: ip}},
		},
		IPAddresses:        []net.IP{ip},
		PublicKeyAlgorithm: 16999792,
	}
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, orgPubkey, orgPrivkey)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	c, err := x509.ParseCertificate(cert)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk})
	pool := x509.NewCertPool()
	pool.AddCert(c)
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	return pool, certificate, err
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
