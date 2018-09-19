package aposerver

import (
	"apollo/data"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/juju/errors.git"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"
)

const TlsTableName = "cert_store"

type TlsManager struct {
	store data.KVStore

	TLSHost string
	TLSPort int

	needToDelete            bool
	TLSCertFile, TLSKeyFile string
	OurCert                 string
}

type TlsData struct {
	Key string
	CertData string
	KeyData string
}

func NewTlsManager() *TlsManager {
	return &TlsManager{}
}

func (man *TlsManager) writeOutKeys(tlsData TlsData) error {
	// We have a certificate/key, store them in temporary files
	keyfile, err := ioutil.TempFile("", "keyfile")
	man.TLSKeyFile = keyfile.Name()
	if err != nil {
		return err
	}
	defer keyfile.Close()
	_, err = keyfile.Write([]byte(tlsData.KeyData))
	if err != nil {
		return err
	}

	certfile, err := ioutil.TempFile("", "certfile")
	if err != nil {
		return err
	}
	man.TLSCertFile = certfile.Name()
	defer certfile.Close()

	_, err = certfile.Write([]byte(tlsData.CertData))
	if err != nil {
		return err
	}
	return nil
}

func (man *TlsManager) probeHost(host string) (string, error) {
	var certs [][]byte
	errObtainedCert := errors.New("Obtained certificate")

	conf := &tls.Config {
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			certs = rawCerts
			return errObtainedCert
		},
	}

	_, err := tls.Dial("tcp", host, conf)
	if err != errObtainedCert {
		return "", err
	}
	if err == nil {
		panic("Unexpected connection success")
	}

	// Now parse the certificate chain to get the CA text
	certBytes := certs[len(certs)-1]
	var keyOut bytes.Buffer
	err = pem.Encode(io.Writer(&keyOut), &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return "", err
	}
	return getCertBody(keyOut.Bytes()), nil
}

func getCertBody(certBytes []byte) string {
	certStr := strings.Replace(string(certBytes), "-----BEGIN CERTIFICATE-----", "", 1)
	certStr = strings.Replace(certStr, "-----END CERTIFICATE-----", "", 1)
	certStr = strings.Replace(certStr, "\n", "", -1)
	return certStr
}

func (man *TlsManager) Init(store data.KVStore, TLSHost string, TLSPort int,
	TLSCert string, TLSKey string, hostToProbe string) error {
	man.TLSHost = TLSHost
	man.TLSPort = TLSPort
	logrus.Info("Setting up TLS infrastructure")

	var err error
	if hostToProbe != "self" {
		logrus.Info("Probing host: %s for CA certificate", hostToProbe)
		man.OurCert, err = man.probeHost(hostToProbe)
		if err != nil {
			return err
		}
	}

	// If no automatic certificate management is desired, just don't do anything
	if TLSCert != "auto" && TLSKey != "auto" {
		logrus.Info("Using manually configured TLS certificate and key")
		man.needToDelete = false
		man.TLSCertFile = TLSCert
		man.TLSKeyFile = TLSKey

		if hostToProbe == "self" {
			file, err := ioutil.ReadFile(TLSCert)
			if err != nil {
				return err
			}
			man.OurCert = getCertBody(file)
		}
		return nil
	}

	man.needToDelete = true
	var tlsData []TlsData
	err = store.LoadTable(TlsTableName, &tlsData)
	if err != nil {
		return nil
	}
	if len(tlsData) > 1 {
		return &ServerError{Err: errors.NewErr("More than one certificate found")}
	} else if len(tlsData) == 1 {
		logrus.Info("Using the stored TLS parameters")
		if hostToProbe == "self" {
			man.OurCert = getCertBody([]byte(tlsData[0].CertData))
		}
		return man.writeOutKeys(tlsData[0])
	}

	logrus.Info("Generating new TLS parameters")
	// New certificate is needed
	certData, err := makeNewCert()
	if err != nil {
		return err
	}
	err = man.writeOutKeys(*certData)
	if err != nil {
		return err
	}

	err, _ = store.StoreValues(TlsTableName, []TlsData{*certData})
	logrus.Info("Saved the generated TLS parameters")
	if err != nil {
		return err
	}

	if hostToProbe == "self" {
		man.OurCert = getCertBody([]byte(certData.CertData))
	}

	return nil
}

func makeNewCert() (*TlsData, error) {

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	// Create a simpel self-signed certificate and key
	template := &x509.Certificate {
		IsCA : true,
		DNSNames: []string{"*"}, // This will allow validation to go through for any name
		BasicConstraintsValid : true,
		SubjectKeyId : []byte{1,2,3},
		SerialNumber : serialNumber,
		Subject : pkix.Name{
			Country : []string{"N/A"},
			Organization: []string{"Apollo"},
		},
		NotBefore : time.Now(),
		NotAfter : time.Now().AddDate(50,0,0),
		// see http://golang.org/pkg/crypto/x509/#KeyUsage
		ExtKeyUsage : []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage : x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign,
	}

	// generate private key
	privatekey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	publickey := &privatekey.PublicKey

	// create a self-signed certificate. template = parent
	var parent = template
	cert, err := x509.CreateCertificate(rand.Reader, template, parent, publickey,privatekey)
	if err != nil {
		return nil, err
	}
	var certOut bytes.Buffer
	err = pem.Encode(io.Writer(&certOut), &pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err != nil {
		return nil, err
	}

	// save private key
	privateKeyBytes, err := x509.MarshalECPrivateKey(privatekey)
	if err != nil {
		return nil, err
	}
	var keyOut bytes.Buffer
	err = pem.Encode(io.Writer(&keyOut), &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})
	if err != nil {
		return nil, err
	}

	return &TlsData{
		Key: "server",
		CertData: string(certOut.Bytes()),
		KeyData: string(keyOut.Bytes()),
	}, nil
}

func (man *TlsManager) Close() {
	if man.needToDelete {
		os.Remove(man.TLSCertFile)
		os.Remove(man.TLSKeyFile)
		man.needToDelete = false
	}
}
