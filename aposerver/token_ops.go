package aposerver

import (
	"apollo/data"
	"apollo/proto/gen/models"
	"apollo/proto/gen/restapi/operations/login"
	"apollo/proto/sigv4sec"
	. "apollo/utils"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/nacl/box"
	"net/http"
	"time"
)

const UserTokenValidDuration = 24 * time.Hour

type LoginProcessor struct {
	ctx context.Context
	aws aws.Config
	store *data.TokenStore
	nodeStore *data.NodeStore
	serverCert string
	whitelistedAccounts map[string]string
	params login.PostSigv4LoginParams
}

func (l *LoginProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed login: %+v", err.Error())
	return login.NewPostSigv4LoginDefault(http.StatusUnauthorized).WithPayload(&models.Error{
		Code: http.StatusUnauthorized, Message: err.Error(),
		RequestID: GetReqIdFromContext(l.ctx)})
}

// Authorize the SigV4 signed request using and create the appropriate token.
// If the request is made from an instance profile we automatically create a
// node-linked token. TODO: don't actually do this?
func (l *LoginProcessor) Enact() middleware.Responder {
	CL(l.ctx).Infof("Invoking LoginProcessor")

	tokenBytes, err := base64.StdEncoding.DecodeString(l.params.Token)
	if err != nil {
		return l.respondWithError(err)
	}
	auth, err := sigv4sec.AuthenticateUser(tokenBytes, l.aws, l.whitelistedAccounts)
	if err != nil {
		return l.respondWithError(err)
	}
	CL(l.ctx).Infof("User %s authenticated successfully", auth.AccountId)

	var entity = ""
	var tokenType data.TokenType
	var validUntil data.AbsoluteTime
	if auth.NodeId == "" {
		entity = auth.AccountId
		tokenType = data.UserToken
		validUntil = data.FromTime(time.Now().Add(UserTokenValidDuration))
	} else {
		entity = auth.NodeId
		tokenType = data.NodeToken
		validUntil = data.NeverExpires // Node tokens are reaped once the node dies

		// Lock the node table to make sure the node doesn't go away
		l.nodeStore.WriteLock()
		defer l.nodeStore.WriteUnlock()
		nodes := l.nodeStore.ListNodes([]string{}, func(node *data.StoredNode) bool {
			return node.CloudID == auth.NodeId
		})
		if len(nodes) == 0 {
			return l.respondWithError(fmt.Errorf("node with id %s is not registered",
				auth.NodeId))
		}
	}

	// Create a user or node token
	token := data.AuthToken {
		Key:     *GenerateRandIdSized(16),
		Expires: validUntil,
		Type:    tokenType,
		EntityKey: entity,
		RequestedBy: "account/"+auth.AccountId,
		RequestedOn: data.FromTime(time.Now()),
	}
	err = l.store.StoreToken(token)
	if err != nil {
		return l.respondWithError(err)
	}

	senderPublicKey, senderPrivateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	// Ok, the user is good. Let's use their pubkey to sign the welcome request!
	greeting := login.PostSigv4LoginOKBody{
		EncryptedAuthToken:   EncryptMessage(token.Key, auth.PublicKey, senderPrivateKey),
		EncryptedCertificate: EncryptMessage(l.serverCert, auth.PublicKey, senderPrivateKey),
		ServerPublicKey:      base64.StdEncoding.EncodeToString((*senderPublicKey)[:]),
		ValidUntil:           strfmt.DateTime(validUntil.ToTime()), // TODO: must be encrypted as well
	}
	return login.NewPostSigv4LoginOK().WithPayload(&greeting)
}


type GetNodeTokenProcessor struct {
	ctx context.Context
	store *data.TokenStore
	serverCert string
	principal data.AuthToken
	params login.GetNodeTokenParams
}

func (l *GetNodeTokenProcessor) respondWithError(err error) middleware.Responder {
	logrus.Warnf("Failed to get node token: %+v", err.Error())
	return login.NewGetNodeTokenDefault(http.StatusUnauthorized).WithPayload(&models.Error{
		Code: http.StatusUnauthorized, Message: err.Error(),
		RequestID: GetReqIdFromContext(l.ctx)})
}

func (l *GetNodeTokenProcessor) Enact() middleware.Responder {
	CL(l.ctx).Infof("Invoking GetNodeTokenProcessor")
	// TODO: validate the token's node

	// Create an instance token
	token := data.AuthToken {
		Key:     *GenerateRandIdSized(16),
		Expires: data.NeverExpires,
		Type:    data.NodeToken,
		EntityKey: *l.params.NodeID,
		RequestedBy: l.principal.RenderEntity(),
		RequestedOn: data.FromTime(time.Now()),
	}
	CL(l.ctx).Infof("Storing token: %s", token.String())

	err := l.store.StoreToken(token)
	if err != nil {
		return l.respondWithError(err)
	}

	greeting := login.GetNodeTokenOKBody{
		AuthToken: token.Key,
		Certificate: l.serverCert,
	}
	return login.NewGetNodeTokenOK().WithPayload(&greeting)
}
