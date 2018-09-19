package data

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/dynamodbattribute"
	"github.com/sirupsen/logrus"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type KVStore interface {
	// Store values in the database using batch writes
	// Returns: error, keys[]
	// keys[] are the list of successfully inserted keys
	StoreValues(table string, data interface{}) (error, map[string]bool)

	// Delete the value from the database, if the item
	// doesn't exist it's a no-op.
	DeleteValue(table string, key string) error

	// Load the entire table into the "output" parameter. It must be a
	// pointer to a slice:
	//
	// 	var data []TestTaskData
	//	err = store.LoadTable("table1", &data)
	//
	// We use reflection to preserve type safety
	LoadTable(table string, output interface{}) error

	// Get the next value of a strictly monotonically increasing
	// sequence with the name counterName.
	// Sequences are created automatically and start with "1".
	GetCounter(counterName string) (int64, error)

	// Init schema, creating the missing tables.
	// Params: map of table names to the expected operations per second.
	InitSchema(tables map[string]int64) error
}

type counterValue struct {
	curVal, maxVal int64
	mutex sync.Mutex
}

type DynamoDBStore struct {
	Svc *dynamodb.DynamoDB
	TablePrefix string

	counterMutex sync.Mutex
	counters map[string]*counterValue
}

const numParallel = 5
const dynamoBatchSize = 25
const keyAttributeName = "Key"
const counterIops = 20
const counterTableName = "counter"
const counterBlockSize = 50

func NewDynamoDbStore(db *dynamodb.DynamoDB, tablePrefix string) KVStore {
	return &DynamoDBStore{
		Svc: db,
		TablePrefix: tablePrefix,
		counters: make(map[string]*counterValue),
	}
}

func (db *DynamoDBStore) StoreValues(table string, data interface{}) (error, map[string]bool) {
	val := reflect.ValueOf(data)
	if val.Kind() != reflect.Array && val.Kind() != reflect.Slice {
		panic("A slice or an array is expected")
	}

	var success = make(map[string]bool)

	counter := 0
	for {
		var requests []dynamodb.WriteRequest
		for i:=0; i<dynamoBatchSize && counter < val.Len(); i++ {
			curVal := val.Index(counter).Interface()
			counter++

			item, err := dynamodbattribute.MarshalMap(curVal)
			if err != nil {
				return err, success
			}

			wr := dynamodb.WriteRequest{
				PutRequest: &dynamodb.PutRequest{Item: item},
			}
			requests = append(requests, wr)
		}

		if len(requests) == 0 {
			break
		}

		input := dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]dynamodb.WriteRequest{
				db.TablePrefix + table: requests}}
		
		for {
			resp, err := db.Svc.BatchWriteItemRequest(&input).Send()
			if err != nil {
				return err, success
			}
			for _, i := range requests {
				success[*i.PutRequest.Item[keyAttributeName].S] = true
			}
			input.RequestItems = resp.UnprocessedItems

			if len(resp.UnprocessedItems) != 0 {
				writeRequests := resp.UnprocessedItems[db.TablePrefix+table]
				for _, i := range writeRequests {
					delete(success, *i.PutRequest.Item[keyAttributeName].S)
				}
			} else {
				break
			}
		}
	}

	return nil, success
}

func (db *DynamoDBStore) DeleteValue(table string, key string) error {
	req := db.Svc.DeleteItemRequest(&dynamodb.DeleteItemInput{
		TableName: aws.String(db.TablePrefix + table),
		Key: map[string]dynamodb.AttributeValue{keyAttributeName: {S: &key},},
	})

	_, err := req.Send()
	return err
}

func (db *DynamoDBStore) LoadTable(table string, output interface{}) error {
	var done = make(chan error, numParallel)
	var mutex sync.Mutex

	outputVal := reflect.ValueOf(output)
	if outputVal.Kind() != reflect.Ptr || reflect.Indirect(outputVal).Kind() != reflect.Slice {
		panic("Was expecting a pointer to a slice")
	}

	sliceType := reflect.Indirect(outputVal).Type()
	result := reflect.MakeSlice(sliceType, 0, 0)

	for i := 0; i < numParallel; i++ {
		// Scan the table in a number of parallel threads
		go func(i int){
			var lastKey map[string]dynamodb.AttributeValue

			// You're not expected to understand this
			var res = reflect.MakeSlice(sliceType,0 ,0)
			x := reflect.New(res.Type())
			x.Elem().Set(res)

			for {
				si := dynamodb.ScanInput{
					TableName:      aws.String(db.TablePrefix + table),
					ConsistentRead: aws.Bool(true),
					Segment:        aws.Int64(int64(i)),
					TotalSegments:  aws.Int64(numParallel),
					Limit:          aws.Int64(1000),
				}

				if lastKey != nil {
					si.ExclusiveStartKey = lastKey
				}

				resp, e := db.Svc.ScanRequest(&si).Send()
				if e != nil {
					done <- e
					return
				}

				e = dynamodbattribute.UnmarshalListOfMaps(resp.Items, x.Interface())
				mutex.Lock()
				result = reflect.AppendSlice(result, reflect.Indirect(x))
				mutex.Unlock()

				if e != nil {
					done <- e
					return
				}

				lastKey = resp.LastEvaluatedKey
				if len(lastKey) == 0 {
					break
				}
			}

			done <- nil
		}(i)
	}

	for i := 0; i < numParallel; i++ {
		err := <-done
		if err != nil {
			return err
		}
	}

	outputVal.Elem().Set(result)
	return nil
}

func (db *DynamoDBStore) GetCounter(counterName string) (int64, error) {
	db.counterMutex.Lock()
	cnt, present := db.counters[counterName]
	if !present {
		cnt = &counterValue{}
		db.counters[counterName] = cnt
	}
	db.counterMutex.Unlock()

	cnt.mutex.Lock()
	defer cnt.mutex.Unlock()

	// We still have unused counter numbers, just return them
	if cnt.curVal < cnt.maxVal {
		var res = cnt.curVal
		cnt.curVal++
		return res, nil
	}

	// Need to grab another block of numbers from the DDB.
	// Here we do an atomic increment on the counter field, we'll get the new value
	// in the response.
	input := &dynamodb.UpdateItemInput{
		Key:              map[string]dynamodb.AttributeValue{
			keyAttributeName: {S: aws.String(counterName)}},
		TableName:        aws.String(db.TablePrefix + counterTableName),
		UpdateExpression: aws.String("add CounterValue :val"),
		ExpressionAttributeValues: map[string]dynamodb.AttributeValue{
			":val": {N: aws.String(strconv.Itoa(counterBlockSize))}},
		ReturnValues: "UPDATED_NEW",
	}

	output, e := db.Svc.UpdateItemRequest(input).Send()
	if e != nil {
		return 0, e
	}
	newVal, e := strconv.ParseInt(*output.Attributes["CounterValue"].N, 10, 64)
	if e != nil {
		return 0, e
	}
	if cnt.maxVal >= newVal {
		panic("Counter " + counterName + " is going down")
	}

	cnt.maxVal = newVal
	// Skip the zero value
	if cnt.curVal == 0 {
		cnt.curVal++
	}

	res := cnt.curVal
	cnt.curVal++
	return res, nil
}

func (db *DynamoDBStore) InitSchema(tables map[string]int64) error {
	var tablesToCreate = make(map[string]int64)
	for k, v := range tables {
		tablesToCreate[k] = v
	}
	tablesToCreate[counterTableName] = counterIops // A special table for counters

	logrus.Info("Describing tables")
	lti := dynamodb.ListTablesInput{}
	for {
		output, err := db.Svc.ListTablesRequest(&lti).Send()
		if err != nil {
			return err
		}

		for _, t := range output.TableNames {
			delete(tablesToCreate, strings.Replace(t, db.TablePrefix, "", 1))
		}

		if output.LastEvaluatedTableName == nil {
			break
		}
		lti.ExclusiveStartTableName = output.LastEvaluatedTableName
	}
	if len(tablesToCreate) == 0 {
		logrus.Info("All tables are up-to-date")
	}

	// Now create the missing tables
	for k, iops := range tablesToCreate {
		newTableName := db.TablePrefix + k
		logrus.Infof("Creating table: %s", newTableName)
		request := db.Svc.CreateTableRequest(&dynamodb.CreateTableInput{
			TableName: aws.String(newTableName),
			AttributeDefinitions: []dynamodb.AttributeDefinition{{
				AttributeName: aws.String(keyAttributeName), AttributeType: "S"}},
			KeySchema: []dynamodb.KeySchemaElement{{
				AttributeName: aws.String(keyAttributeName), KeyType: "HASH"}},
			ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
				ReadCapacityUnits: aws.Int64(iops),
				WriteCapacityUnits: aws.Int64(iops),
			},
		})
		_, e := request.Send()
		if e != nil {
			return e
		}

		db.Svc.WaitUntilTableExists(&dynamodb.DescribeTableInput{
			TableName: aws.String(newTableName),
		})
	}

	return nil
}
