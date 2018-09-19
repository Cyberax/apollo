package sigv4sec

import (
	"apollo/proto/sigv4sec/mocks"
	"apollo/utils"
	"bytes"
	"crypto/rand"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/jarcoal/httpmock"
	"github.com/petergtz/pegomock"
	"golang.org/x/crypto/nacl/box"
	. "gopkg.in/check.v1"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	pegomock.RegisterMockTestingT(t)
	TestingT(t)
}
type JugglerTests struct{}
var _ = Suite(&JugglerTests{})

const expectedRequest = `POST / HTTP/1.1
Host: sts.amazonaws.com
User-Agent: Go-http-client/1.1
Content-Length: 43
Accept-Encoding: identity
Authorization: AWS4-HMAC-SHA256 Credential=AKIAIPHI2P3JNDCCADSD/20010909/us-east-1/sts/aws4_request, SignedHeaders=accept-encoding;content-type;host;x-amz-date;x-amz-meta-client-key;x-amz-security-token, Signature=f374d65f21f03047116be654961d40f57a2058383e184d2867040f761dbe3381
Content-Type: application/x-www-form-urlencoded
X-Amz-Date: 20010909T014640Z
X-Amz-Meta-Client-Key: nge5J9gorG0iY4ZUvQPfHg4daRsFlYtspIhBfNVULBA=
X-Amz-Security-Token: ASKJDHKAJSDH

Action=GetCallerIdentity&Version=2011-06-15`


func makeCfg() aws.Config {
	// Not real credentials, btw.
	cfg, e := external.LoadDefaultAWSConfig(
		external.WithRegion("us-east-1"),
		external.WithCredentialsValue(aws.Credentials{
			AccessKeyID:     "AKIAIPHI2P3JNDCCADSD",
			SecretAccessKey: "3yR0/hJTWe2EFmixz/AKJHSDGJKAHSGDJKHG",
			SessionToken:    "ASKJDHKAJSDH",
		}))
	if e != nil {
		panic(e.Error())
	}
	return cfg
}

func (s *JugglerTests) TestSignatureCalculation(c *C) {
	cfg := makeCfg()

	ClockFunc = utils.StaticClock(1000000000)
	defer func() {
		ClockFunc = time.Now
	}()

	// Generate a pre-determined keypair
	publicKey, _, _ := box.GenerateKey(strings.NewReader(strings.Repeat("asdf", 1000)))
	req := CreateSignedRequest(cfg, publicKey)

	request, _ := ParseAndValidateRequest(req, cfg)

	// Check that the request matches the known test vector
	var b bytes.Buffer
	request.Write(io.Writer(&b))
	print(b.Bytes())

	httpStr := strings.Replace(expectedRequest, "\n", "\r\n", -1)
	c.Assert(string(b.Bytes()), Equals, httpStr)
}

func (s *JugglerTests) TestAdversarialInput(c *C) {
	_, e := ParseAndValidateRequest([]byte("GARBAGE"), makeCfg())
	c.Assert(e.Error(), Equals, "malformed HTTP request \"GARBAGE\"")

	// Try to a corrupted request with a fake action data
	badInput := strings.Replace(expectedRequest,
		"Action=GetCallerIdentity&Version=2011-06-15",
		"Action=TerminateAllInsts&Version=2011-06-15", -1)

	req, _ := ParseAndValidateRequest([]byte(badInput), makeCfg())

	// Check that the request matches the known test vector
	// and the malicious input is erased.
	var b bytes.Buffer
	req.Write(io.Writer(&b))
	print(b.Bytes())

	httpStr := strings.Replace(expectedRequest, "\n", "\r\n", -1)
	c.Assert(string(b.Bytes()), Equals, httpStr)
}

func (s *JugglerTests) TestUserAuthentication(c *C) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	er := mocks.NewMockEndpointResolver()
	pegomock.When(er.ResolveEndpoint("sts", "us-mars-1")).
		ThenReturn(aws.Endpoint{URL: "https://sts.endpoint.yes", SigningRegion: "us-mars-1"}, nil)

	cfg := aws.Config{
		Credentials:      aws.NewStaticCredentialsProvider("key1", "secret1", ""),
		EndpointResolver: er,
		Region:           "us-mars-1",
		HTTPClient:       http.DefaultClient,
	}

	httpmock.RegisterResponder("POST", "https://sts.endpoint.yes",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(200, `
<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::158005755667:user/cyberax</Arn>
    <UserId>AIDAJJGHH5Y53VXNHXHNG:i-1232341asdkjf</UserId>
    <Account>158005755667</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata>
    <RequestId>eb5d8b58-9e18-11e8-b32c-b77fbbd26035</RequestId>
  </ResponseMetadata>
</GetCallerIdentityResponse>`)
			return resp, nil
		},
	)

	publicKey, _, _ := box.GenerateKey(rand.Reader)
	req := CreateSignedRequest(cfg, publicKey)

	// Happy case
	auth, e := AuthenticateUser(req, cfg, map[string]string{"158005755667": "158005755667"})
	c.Assert(e, Equals, nil)
	c.Assert(bytes.Equal((*auth.PublicKey)[:], publicKey[:]), Equals, true)

	// Check that a non-whitelisted user is disallowed
	_, e = AuthenticateUser(req, cfg, map[string]string{"12341234": "12341234"})
	c.Assert(e, Equals, ErrUserUnauthorized)

	// Try a request without a key
	reqWithoutKey := bytes.Replace(req, []byte(PublicKeyKey), []byte("Bad"), -1)
	_, e = AuthenticateUser(reqWithoutKey, cfg, map[string]string{"158005755667": "158005755667"})
	c.Assert(e, Equals, ErrUserUnauthorized)

	httpmock.RegisterResponder("POST", "https://sts.endpoint.yes",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(401, `
<ErrorResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <Error>
    <Type>Sender</Type>
    <Code>InvalidClientTokenId</Code>
    <Message>The security token included in the request is invalid.</Message>
  </Error>
  <RequestId>cb8d1c0c-9eb2-11e8-8f77-515da1a9422f</RequestId>
</ErrorResponse>
`)
			return resp, nil
		})

	// Check for Amazon error handling
	_, e = AuthenticateUser(req, cfg, map[string]string{"12341234": "12341234"})
	awsErr := e.(awserr.Error)
	c.Assert(awsErr.Code(), Equals, "InvalidClientTokenId")
	c.Assert(awsErr.Message(), Equals, "The security token included in the request is invalid.")
	c.Assert(e.(awserr.RequestFailure).StatusCode(), Equals, 401)

	httpmock.RegisterResponder("POST", "https://sts.endpoint.yes",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(401, `
<ServiceUnavailableException xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <RequestId>cb8d1c0c-9eb2-11e8-8f77-515da1a9422f</RequestId>
</ServiceUnavailableException>
`)
			return resp, nil
		})

	// Check for Amazon error handling
	_, e = AuthenticateUser(req, cfg, map[string]string{"12341234": "12341234"})
	awsErr = e.(awserr.Error)
	c.Assert(awsErr.Code(), Equals, "ServiceUnavailableException")

	// Test for corrupted HTTP responses
	httpmock.RegisterResponder("POST", "https://sts.endpoint.yes",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(401, "baderr")
			return resp, nil
		})

	_, e = AuthenticateUser(req, cfg, map[string]string{"12341234": "12341234"})
	awsErr = e.(awserr.Error)
	c.Assert(awsErr.Code(), Equals, "SerializationError")
}
