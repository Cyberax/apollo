package gen

import (
	"apollo/proto/gen/models"
	"github.com/sirupsen/logrus"
	"reflect"
)

//go:generate go run ../swagger/mergeyaml.go ../swagger/swagger.yaml merged.yaml
//go:generate go run ../../vendor/github.com/go-swagger/go-swagger/cmd/swagger/swagger.go generate server --exclude-main -f merged.yaml
//go:generate go run ../../vendor/github.com/go-swagger/go-swagger/cmd/swagger/swagger.go generate client  -c restcli --api-package cliops -f merged.yaml


func PrintError(err error) {
	// Is this a structured error?
	errValue := reflect.ValueOf(err)
	if errValue.Kind() != reflect.Ptr {
		logrus.Error(err)
		return
	}

	fieldValue := errValue.Elem().FieldByName("Payload")
	if fieldValue == (reflect.Value{}) {
		logrus.Error(err)
		return
	}

	errorInfo := fieldValue.Interface().(*models.Error)
	if errorInfo == nil {
		logrus.Error(err)
		return
	}

	logrus.Errorf("%+v", *errorInfo)
	return
}
