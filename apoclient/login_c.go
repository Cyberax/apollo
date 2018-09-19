package apoclient

import (
	"apollo/proto/gen/restcli"
	"apollo/proto/gen/restcli/login"
	"apollo/proto/sigv4sec"
	"apollo/utils"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/go-openapi/runtime"
	client2 "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/juju/errors.git"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/nacl/box"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const userDataMarker = "### APOLLO_SERVER_URL IS "
const metadataUrl = "http://169.254.169.254/latest/user-data"
const ApolloConnectionKey = "APOLLO_CONNECTION"

type LoginData struct {
	token string
	expires time.Time
	serverCert tls.Certificate
}

type LoginError struct {
	errors.Err
}

func MakeLoginCmd() *cobra.Command {
	var cmdLogin = &cobra.Command{
		Use:          "login",
		Short:        "Login and get the session token",
		Long:         `login will use your AWS credentials to obtain a session token from the Apollo server`,
		Args:         cobra.MinimumNArgs(0),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return DoAwsLogin(cmd.Flag("profile").Value.String(),
				cmd.Flag("host").Value.String(),
				cmd.Flag("token-file").Value.String())
		},
	}
	cmdLogin.Flags().SortFlags = false
	cmdLogin.Flags().StringP("profile", "p", "default", "AWS profile")
	cmdLogin.Flags().StringP("host", "s", "", "Server's host and port")
	return cmdLogin
}

func LookupServerFromUserData() string {
	client := http.Client{}
	client.Timeout = 2 * time.Second
	resp, err := client.Get(metadataUrl)
	if err != nil {
		return ""
	}

	userData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	for _, st := range strings.Split(string(userData), "\n") {
		if strings.Index(st, userDataMarker) == 0 {
			return strings.TrimSpace(st[len(userDataMarker):])
		}
	}
	return ""
}

type HeaderAddingRtt struct {
}

func (t *HeaderAddingRtt) AuthenticateRequest(request runtime.ClientRequest,
	r strfmt.Registry) error {
	request.SetHeaderParam("X-Request-Id", *utils.GenerateRandId())
	return nil
}

type SigV4Res struct {
	AuthToken string
	ServerCert string
}

func SendSigv4Auth(awsProfile, url string) (SigV4Res, error) {
	if url == "" {
		url = LookupServerFromUserData()
	}
	if url == "" {
		return SigV4Res{}, &LoginError{errors.NewErr("No URL is provided and it can't discovered from user-data")}
	}

	if awsProfile == "" {
		awsProfile = "default"
	}

	// Load the AWS keys that we're going to use to authenticate
	config, e := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile(awsProfile))
	if e != nil {
		return SigV4Res{}, e
	}

	// Generate the key that the server is going to use to encrypt the token sent to us
	publicKey, privKey, e := box.GenerateKey(rand.Reader)
	if e != nil {
		return SigV4Res{}, e
	}

	request := sigv4sec.CreateSignedRequest(config, publicKey)

	// The Login is a special method - we accept any certificate from the server.
	client, e := client2.TLSClient(client2.TLSClientOptions{InsecureSkipVerify: true})
	if e != nil {
		return SigV4Res{}, e
	}
	trans := client2.NewWithClient(url, "", []string{"https"}, client)
	trans.DefaultAuthentication = &HeaderAddingRtt{}
	cli := restcli.New(trans, nil)

	pa := login.NewPostSigv4LoginParams()
	pa.Token = base64.StdEncoding.EncodeToString(request)
	greeting, e := cli.Login.PostSigv4Login(pa)
	if e != nil {
		return SigV4Res{}, e
	}

	payload := greeting.Payload
	authToken, ok := utils.DecryptMessage(payload.EncryptedAuthToken,
		payload.ServerPublicKey, privKey)
	if !ok {
		return SigV4Res{}, &LoginError{errors.NewErr("Failed to open the secure box")}
	}

	serverCert, ok := utils.DecryptMessage(payload.EncryptedCertificate,
		payload.ServerPublicKey, privKey)
	if !ok {
		return SigV4Res{}, &LoginError{errors.NewErr("Failed to open the secure box")}
	}

	return SigV4Res{
		AuthToken: authToken,
		ServerCert: serverCert,
	}, nil
}

func DoAwsLogin(awsProfile, url, targetFile string) error {
	if os.Getenv(ApolloConnectionKey) != "" {
		logrus.Warn("Found APOLLO_CONNECTION in the environment, it will take precedence.")
	}

	res, err := SendSigv4Auth(awsProfile, url)
	if err != nil {
		return err
	}

	logrus.Debugf("Received a successful connection token")
	// Print the actual result to stdout
	fmt.Printf("APIKEY\t%s\n", res.AuthToken)

	savedToken := url + "#" + res.AuthToken + "#" + res.ServerCert
	return ioutil.WriteFile(targetFile, []byte(savedToken), 0600)
}


func MakeGetNodeTokenCmd() *cobra.Command {
	var cmdLogin = &cobra.Command{
		Use:          "get-node-token node-id",
		Short:        "Make a node-specific authentication token",
		Long:         `Get a token for node-linked authentication`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, token, err := ObtainConnectionWithInfo(cmd)
			if err != nil {
				return err
			}

			return DoGetNodeToken(conn, token.Host, args[0])
		},
	}
	return cmdLogin
}

func DoGetNodeToken(cli *restcli.Apollo, host string, nodeId string) error {
	params := login.NewGetNodeTokenParams()
	params.NodeID = &nodeId

	res, err := cli.Login.GetNodeToken(params, nil)
	if err != nil {
		return err
	}

	fmt.Printf("TOKEN\t"+host+"#"+res.Payload.AuthToken+"#"+res.Payload.Certificate+"\n")
	return nil
}


func MakePingCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "ping",
		Short:        "Ping the server",
		Long:         `Check connectivity with the server`,
		Args:         cobra.MinimumNArgs(0),
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, e := ObtainConnection(cmd)
			if e != nil {
				return e
			}
			_, e = connection.Login.GetPing(login.NewGetPingParams(), nil)
			if e == nil {
				print("OK")
			}
			return e
		},
	}
}
