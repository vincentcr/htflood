package req

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

var transport *http.Transport = createTransport()

func createTransport() *http.Transport {

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		MaxIdleConnsPerHost: 512,
		TLSClientConfig:     &tls.Config{},
	}

	return transport
}

func loadSelfSignedCertificate() {
	// server cert is self signed -> server_cert == ca_cert
	CA_Pool := x509.NewCertPool()
	severCert, err := ioutil.ReadFile("./tls-cert.pem")
	if err != nil {
		log.Fatal("Could not load server certificate!")
	}
	CA_Pool.AppendCertsFromPEM(severCert)
}
