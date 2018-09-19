package aposerver

import (
	"apollo/proto/sigv4sec"
	"github.com/aws/aws-sdk-go-v2/aws"
	"apollo/data"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/juju/errors.git"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type ServerError struct {
	errors.Err
}

type ServerContext struct {
	Verbose bool
	AwsConfig aws.Config
	KvStore data.KVStore

	// Dependency registry
	// Token store
	TlsManager *TlsManager
	TokenStore *data.TokenStore
	TaskStore *data.TaskStore
	QueueStore *data.QueueStore
	NodeStore *data.NodeStore
	WhitelistedAccounts map[string]string
}

func (ctx* ServerContext) InitRegistry(v *viper.Viper) error {
	logrus.Info("Initializing the registry")

	// Create the AWS context
	AwsConfig, err := external.LoadDefaultAWSConfig(
		external.WithSharedConfigProfile(v.GetString("aws.profile")),
		external.WithRegion(v.GetString("aws.region")),
	)
	if err != nil {
		return err
	}
	ctx.AwsConfig = AwsConfig

	// Create the database
	storeType := v.GetString("database.type")
	logrus.Infof("Using store type: %s", storeType)
	switch storeType {
	case "ddb":
		ctx.KvStore = data.NewDynamoDbStore(
			dynamodb.New(ctx.AwsConfig), v.GetString("database.prefix"))
	case "mem":
		ctx.KvStore = data.NewFakeMemStore()
	default:
		return data.NewStoreError("Unknown store type " + storeType, nil)
	}

	// Create schema
	logrus.Info("Initializing the schema")
	err = ctx.KvStore.InitSchema(map[string]int64 {
		TlsTableName: 5,
		data.TokenStoreTable: 10,
		data.TaskTable: 10,
		data.TaskInstanceTable: 10,
		data.QueueTable: 5,
		data.NodeTable: 5,
	})
	if err != nil {
		return err
	}

	// Build the TLS manager
	ctx.TlsManager = NewTlsManager()
	err = ctx.TlsManager.Init(ctx.KvStore, v.GetString("listen.interface"),
		v.GetInt("listen.port"),
		v.GetString("listen.certfile"),
		v.GetString("listen.keyfile"),
		v.GetString("listen.probe-host"))
	if err != nil {
		return err
	}

	// Token store
	ctx.TokenStore = data.NewTokenStore(ctx.KvStore)
	// Task store
	ctx.TaskStore = data.NewTaskStore(ctx.KvStore)
	// Queue store
	ctx.QueueStore = data.NewQueueStore(ctx.KvStore)
	// Node store
	ctx.NodeStore = data.NewNodeStore(ctx.KvStore)

	// Whitelisted accounts
	ctx.WhitelistedAccounts = make(map[string]string)
	for _, acct := range v.GetStringSlice("server.whitelisted-accounts") {
		if acct == "self" {
			acct, err = sigv4sec.GetMyAccountId(ctx.AwsConfig)
			if err != nil {
				return err
			}
		}
		ctx.WhitelistedAccounts[acct] = acct
	}

	logrus.Info("Hydrating the in-memory stores")
	ctx.TokenStore.Hydrate()
	ctx.TaskStore.Hydrate()
	ctx.QueueStore.Hydrate()
	ctx.NodeStore.Hydrate()

	return nil
}

func (ctx *ServerContext) Close() {
	if ctx.TlsManager != nil {
		ctx.TlsManager.Close()
	}
}
