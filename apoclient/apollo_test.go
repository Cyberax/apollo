package apoclient

import (
	"github.com/jarcoal/httpmock"
	"github.com/petergtz/pegomock"
	. "gopkg.in/check.v1"
	"net/http"
	"testing"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	pegomock.RegisterMockTestingT(t)
	TestingT(t)
}
type ApolloClientTests struct{}
var _ = Suite(&ApolloClientTests{})

func (s *ApolloClientTests) TestUrlDiscovery(c *C) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Check that environment-based discovery works
	url := LookupServerFromUserData()

	httpmock.RegisterResponder("GET", "http://169.254.169.254/latest/user-data",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(200, `#!/bin/bash
test-metdata
might-do-something-here
### APOLLO_SERVER_URL IS http://somewhere.com
`)
			return resp, nil
		},
	)

	url = LookupServerFromUserData()
	c.Assert(url, Equals, "http://somewhere.com")

	httpmock.RegisterResponder("GET", "http://169.254.169.254/latest/user-data",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(200, `#!/bin/bash
test-metdata
no-url-here
`)
			return resp, nil
		},
	)

	url = LookupServerFromUserData()
	c.Assert(url, Equals, "")

	httpmock.RegisterResponder("GET", "http://169.254.169.254/latest/user-data",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(404, ""), nil
		},
	)
	url = LookupServerFromUserData()
	c.Assert(url, Equals, "")
}
