package tpu_quic

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/desperatee/solana-go"
	"log"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	applicationNoError = "Application error 0x0"
	closedConnError    = "use of closed network connection"
	noActivity         = "recent network activity"
)

func isPeerGoingAway(err error) bool {
	if err == nil {
		return false
	}
	str := err.Error()

	if strings.Contains(str, closedConnError) ||
		strings.Contains(str, applicationNoError) ||
		strings.Contains(str, noActivity) {
		return true
	} else {
		return false
	}
}

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

func NewSelfSignedTLSCertificate(ip net.IP) (tls.Certificate, error) {
	wallet := solana.NewWallet()
	key := ed25519.PrivateKey(wallet.PrivateKey[:])

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}
	notAfter := time.Now().Add(24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "Solana node",
		},
		NotBefore:             time.Time{},
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{ip},
		BasicConstraintsValid: true,
		PublicKeyAlgorithm:    16999792,
	}
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{cert}, PrivateKey: key}, nil
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
