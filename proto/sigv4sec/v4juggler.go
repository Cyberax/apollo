package sigv4sec

import (
	"apollo/utils"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/private/protocol/xml/xmlutil"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/juju/errors.git"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ClockFunc = time.Now

const GetIdentityReq = "Action=GetCallerIdentity&Version=2011-06-15"
const PublicKeyKey = "X-Amz-Meta-Client-Key"
var ErrUserUnauthorized = errors.New("unauthorized user")

func mustParse(urlToParse string) *url.URL {
	stsUrl, e := url.Parse(urlToParse)
	if e != nil {
		panic(e)
	}
	return stsUrl
}

// Create a signed HTTP request of a GetCallerIdentity AWS call. This request
// additionally contains the key that'll be used to encrypt the returned token
func CreateSignedRequest(cfg aws.Config, key utils.EC25519PublicKey) []byte {
	stsEndpoint, e := cfg.EndpointResolver.ResolveEndpoint("sts", cfg.Region)
	if e != nil {
		panic(e.Error())
	}

	request, e := http.NewRequest("POST", stsEndpoint.URL,
		strings.NewReader(GetIdentityReq))
	if e != nil {
		panic(e)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Accept-Encoding", "identity")

	// Add our public key as a meta key
	request.Header.Add(PublicKeyKey, base64.StdEncoding.EncodeToString((*key)[:]))

	// Compute the actual signature
	_, e = v4.NewSigner(cfg.Credentials).Sign(request, strings.NewReader(GetIdentityReq),
		"sts", stsEndpoint.SigningRegion, ClockFunc())
	if e != nil {
		panic(e)
	}

	var b bytes.Buffer
	request.Write(io.Writer(&b))
	return b.Bytes()
}

func ParseAndValidateRequest(request []byte, config aws.Config) (*http.Request, error) {
	stsEndpoint, e := config.EndpointResolver.ResolveEndpoint("sts", config.Region)
	if e != nil {
		panic(e.Error())
	}

	parsedReq, e := http.ReadRequest(bufio.NewReader(bytes.NewReader(request)))
	if e != nil {
		return nil, e
	}

	// Validate the request now. We're not stupid and won't let you use
	// our code as a proxy.
	// Rewrite the URL with a known good one
	parsedReq.Body = ioutil.NopCloser(strings.NewReader(GetIdentityReq))

	// Rewrite the endpoint as well
	parsedReq.URL = mustParse(stsEndpoint.URL)
	parsedReq.RequestURI = ""
	return parsedReq, nil
}

type XmlErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Code      string   `xml:"Error>Code"`
	Message   string   `xml:"Error>Message"`
	RequestID string   `xml:"RequestId"`
}

type XmlServiceUnavailableResponse struct {
	XMLName xml.Name `xml:"ServiceUnavailableException"`
}

func decodeAwsError(bodyData []byte, response *http.Response) awserr.Error {
	var errDesc = XmlErrorResponse{}
	e := xml.NewDecoder(bytes.NewReader(bodyData)).Decode(&errDesc)
	if e == nil {
		ae := awserr.New(errDesc.Code, errDesc.Message, nil)
		return awserr.NewRequestFailure(ae, response.StatusCode, errDesc.RequestID)
	}

	var errUnavail = XmlServiceUnavailableResponse{}
	e = xml.NewDecoder(bytes.NewReader(bodyData)).Decode(&errUnavail)
	reqId := response.Header.Get("x-amz-request-id")
	if e == nil {
		return awserr.NewRequestFailure(
			awserr.New("ServiceUnavailableException", "service is unavailable", nil),
			response.StatusCode, reqId)
	}
	return awserr.NewRequestFailure(awserr.New("SerializationError",
		"Failed to parse the response", nil), response.StatusCode, reqId)
}

func GetMyAccountId(sess aws.Config) (string, error) {
	stsApi := sts.New(sess)
	output, e := stsApi.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{}).Send()
	if e != nil {
		return "", e
	}
	return *output.Account, nil
}

type AuthenticatedUser struct {
	PublicKey utils.EC25519PublicKey
	AccountId string
	NodeId string
}

// Authenticate the user based on a shipped AWS authentication request.
// Returns the Ed25519 public key to use to ship our certificate back to the user
func AuthenticateUser(request []byte, config aws.Config,
	whitelistedAccounts map[string]string) (AuthenticatedUser, error) {

	validatedRequest, e := ParseAndValidateRequest(request, config)
	if e != nil {
		return AuthenticatedUser{}, e
	}

	response, e := config.HTTPClient.Do(validatedRequest)
	if e != nil {
		return AuthenticatedUser{}, e
	}
	defer response.Body.Close()

	var out = sts.GetCallerIdentityOutput{}
	var bodyData bytes.Buffer
	bodyData.ReadFrom(response.Body)
	bodyBytes := bodyData.Bytes()

	decoder := xml.NewDecoder(bytes.NewReader(bodyBytes))
	e = xmlutil.UnmarshalXML(&out, decoder, "GetCallerIdentityResult")

	if e != nil || out.Account == nil {
		// An error during unmarshalling - likely we got an exception from the
		// server.
		return AuthenticatedUser{}, decodeAwsError(bodyBytes, response)
	}

	if _, present := whitelistedAccounts[*out.Account]; !present {
		return AuthenticatedUser{}, ErrUserUnauthorized
	}

	pubKey := validatedRequest.Header.Get(PublicKeyKey)
	if pubKey == "" {
		return AuthenticatedUser{}, ErrUserUnauthorized
	}

	pubKeyDecoded, e := base64.StdEncoding.DecodeString(pubKey)
	if e != nil {
		return AuthenticatedUser{}, e
	}
	var pk [32]byte
	copy(pk[:], pubKeyDecoded)

	var nodeId = ""

	splitString := strings.SplitN(*out.UserId, ":", 2)
	if len(splitString) == 2 && strings.HasPrefix(splitString[1], "i-") {
		nodeId = splitString[1]
	}

	res := AuthenticatedUser{
		PublicKey: utils.EC25519PublicKey(&pk),
		AccountId: *out.Account,
		NodeId: nodeId,
	}
	return res, nil
}
