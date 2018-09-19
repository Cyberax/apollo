package data

import (
	"testing"
	"os/exec"
	"os"
	"bufio"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	"github.com/stretchr/testify/assert"
	"strconv"
	"apollo/utils"
	"time"
)

type testContext struct {
	ddb *exec.Cmd
	conn *dynamodb.DynamoDB
	port uint16
}

func closeContext(ctx testContext) {
	ctx.ddb.Process.Kill()
	ctx.ddb.Wait()
}

func prepareContext(t *testing.T) testContext {
	// Get a free port
	port, e := utils.GetFreeTcpPort()
	if e != nil {
		t.FailNow()
	}

	// Try to launch the Local DDB
	cmd := exec.Command("java", "-jar", "DynamoDBLocal.jar", "-inMemory",
		"-port", strconv.Itoa(port))
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	cmd.Dir = "../localddb"
	cmd.Stdin = os.Stdin

	e = cmd.Start()
	if e != nil {
		t.Log("Can't launch DDB local")
		t.SkipNow()
	}

	scanner := bufio.NewScanner(out)
	scanner.Split(bufio.ScanWords)
	var found = false
	for {
		scanner.Scan()
		if scanner.Err() != nil {
			t.Log("Can't launch DDB local")
			t.SkipNow()
		}
		if scanner.Text() == "CorsParams:" {
			found = true
			break
		}
	}

	if !found {
		t.Log("Failed to initialize the DDB")
		t.SkipNow()
	}

	config := defaults.Config()
	config.Region = "mock-region"
	config.EndpointResolver = aws.ResolveWithEndpointURL(
		"http://localhost:" + strconv.Itoa(port))
	config.Credentials = aws.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "SESSION",
			Source: "unit test credentials",
		},
	}

	return testContext{
		conn: dynamodb.New(config),
		ddb: cmd,
		port: uint16(port),
	}
}

type TestTaskData struct {
	Key           string
	Cmd           string
	Env           map[string]string
	TimeoutMillis int64
}

func TestKvStore(t *testing.T) {
	context := prepareContext(t)
	defer func() { closeContext(context) }()

	store := NewDynamoDbStore(context.conn, "test_")
	err := store.InitSchema(map[string]int64{"table1": 100, "table2": 200})
	assert.NoError(t, err)

	numItems := 10000
	allData := make([]AuthToken, numItems)
	for i := 0; i < numItems; i++ {
		allData[i] = AuthToken{
			Key:     "key" + strconv.Itoa(i),
			Expires: FromTime(time.Now()),
			Type:    NodeToken,
		}
	}
	err, _ = store.StoreValues("table1", allData)
	assert.NoError(t, err)

	var data []AuthToken
	err = store.LoadTable("table1", &data)
	assert.NoError(t, err)
	assert.Equal(t, numItems, len(data))

	assert.NoError(t, store.DeleteValue("table1", "key1"))
	err = store.LoadTable("table1", &data)
	assert.NoError(t, err)
	assert.Equal(t, numItems-1, len(data))
}

func TestCounters(t *testing.T) {
	context := prepareContext(t)
	defer func() { closeContext(context) }()

	store := NewDynamoDbStore(context.conn, "test_")
	err := store.InitSchema(map[string]int64{})
	assert.NoError(t, err)

	counter, err := store.GetCounter("tasks")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), counter)

	for i := 0; i < 1000; i++ {
		counter, err = store.GetCounter("tasks")
		assert.NoError(t, err)
		assert.Equal(t, int64(i+2), counter)
	}
}

func TestCounterPanic(t *testing.T) {
	type Counter struct {
		Key          string
		CounterValue int64
	}

	context := prepareContext(t)
	defer func() { closeContext(context) }()

	store := NewDynamoDbStore(context.conn, "test_")
	err := store.InitSchema(map[string]int64{})
	assert.NoError(t, err)

	counter, err := store.GetCounter("tasks")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), counter)

	data := []Counter{{Key: "tasks", CounterValue: 0}}
	store.StoreValues("counter", data)

	assert.Panics(t, func() {
		for i := 0; i < 100; i++ {
			store.GetCounter("tasks")
		}
	})
}

func TestFakeMemStore(t *testing.T) {
	store := NewFakeMemStore()

	err := store.InitSchema(map[string]int64{"table1": 100, "table2": 200})
	assert.NoError(t, err)

	for i := 0; i<100; i++ {
		counter, err := store.GetCounter("tasks")
		assert.NoError(t, err)
		assert.Equal(t, int64(i+1), counter)
	}

	numItems := 10000
	allData := make([]TestTaskData, numItems)
	for i := 0; i < numItems; i++ {
		allData[i] = TestTaskData{
			Key:           "key" + strconv.Itoa(i),
			Cmd:           "cmd--" + strconv.Itoa(i),
			Env:           map[string]string{"env1": "env2", "env3": "env4"},
			TimeoutMillis: int64(i),
		}
	}
	err, _ = store.StoreValues("table1", allData)
	assert.NoError(t, err)

	var data []TestTaskData
	err = store.LoadTable("table1", &data)
	assert.NoError(t, err)
	assert.Equal(t, numItems, len(data))
}