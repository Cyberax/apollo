package apoclient

import (
	"apollo/proto/gen/restcli"
	"apollo/utils"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"github.com/go-openapi/runtime"
	client2 "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"strings"
)

type KeyAddingRtt struct {
	token string
}

func (t *KeyAddingRtt) AuthenticateRequest(request runtime.ClientRequest,
	r strfmt.Registry) error {
	request.SetHeaderParam("X-Request-Id", *utils.GenerateRandId())
	request.SetHeaderParam("X-Apollo-Token", t.token)
	return nil
}

func ObtainConnection(cmd *cobra.Command) (*restcli.Apollo, error) {
	apollo, _, err := ObtainConnectionWithInfo(cmd)
	return apollo, err
}

type ApolloTokenInfo struct {
	Host string
	ServerCert *x509.Certificate
	AuthToken string
}

func DecodeTokenString(token string) (ApolloTokenInfo, error) {
	// Format is: host:port#token#cert
	components := strings.Split(token, "#")
	if len(components) != 3 {
		return ApolloTokenInfo{}, fmt.Errorf("incorrect token format in")
	}

	certBytes, err := base64.StdEncoding.DecodeString(components[2])
	if err != nil {
		return ApolloTokenInfo{}, fmt.Errorf("failed to decode the certificate in token")
	}

	certificate, err := x509.ParseCertificate(certBytes)
	if err != nil {
		logrus.Debugf(err.Error())
		return ApolloTokenInfo{}, fmt.Errorf("failed to parse the certificate in the token")
	}

	return ApolloTokenInfo{
		Host: components[0],
		AuthToken: components[1],
		ServerCert: certificate,
	}, nil
}

func MakeConnection(token ApolloTokenInfo) (*restcli.Apollo, error) {
	// The Login is a special method - we accept any certificate from the server.
	client, e := client2.TLSClient(client2.TLSClientOptions{
		InsecureSkipVerify: false,
		LoadedCA: token.ServerCert,
	})
	if e != nil {
		return nil, e
	}

	trans := client2.NewWithClient(token.Host, "", []string{"https"}, client)
	trans.DefaultAuthentication = &KeyAddingRtt{token: token.AuthToken}
	trans.EnableConnectionReuse() // We reuse the connection to avoid HTTPS handshakes

	return restcli.New(trans, nil), nil
}

func ObtainConnectionWithInfo(cmd *cobra.Command) (*restcli.Apollo, *ApolloTokenInfo, error) {
	tokenBytes := []byte(os.Getenv(ApolloConnectionKey))

	if len(tokenBytes) == 0 {
		tokenPath := utils.GetFlagS(cmd, "token-file")
		if tokenPath == "" {
			return nil, nil, fmt.Errorf("couldn't find the token file and no APOLLO_CONNECTION specified")
		}

		var e error
		tokenBytes, e = ioutil.ReadFile(tokenPath)
		if e != nil {
			return nil, nil, fmt.Errorf("failed to read the token file: %s", tokenPath)
		}
	}

	token, err := DecodeTokenString(string(tokenBytes))
	if err != nil {
		return nil, nil, err
	}

	cli, err := MakeConnection(token)
	if err != nil {
		return nil, nil, err
	}

	return cli, &token, nil
}
