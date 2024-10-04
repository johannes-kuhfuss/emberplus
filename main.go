package main

import (
	"fmt"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/client"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/services_utils/logger"
)

var ()

func main() {
	logger.Info("Starting...")

	ec, _ := client.NewEmberClient("127.0.0.1", 8999)
	ec.Connect()
	rr, err := ember.GetRootRequest()
	if err != nil {
		logger.Error("error getting root request", err)
	} else {
		ec.Write(rr)
	}
	out, err := ec.Receive()
	if err != nil {
		logger.Error("error receiving answer", err)
	}
	el := ember.NewElementConnection()
	err = el.Populate(asn1.NewDecoder(out))
	if err != nil {
		logger.Error("error processing answer", err)
	}
	data, err := el.MarshalJSON()
	if err != nil {
		logger.Error("error marshalling answer", err)
	}
	logger.Info(fmt.Sprintf("data: %v", string(data)))
	r2, err := ember.GetRequestByType("node", "0.2")
	if err != nil {
		logger.Error("error getting root request", err)
	} else {
		ec.Write(r2)
	}
	out, err = ec.Receive()
	if err != nil {
		logger.Error("error receiving answer", err)
	}
	el2 := ember.NewElementConnection()
	err = el2.Populate(asn1.NewDecoder(out))
	if err != nil {
		logger.Error("error processing answer", err)
	}
	data, err = el2.MarshalJSON()
	if err != nil {
		logger.Error("error marshalling answer", err)
	}
	logger.Info(fmt.Sprintf("data: %v", string(data)))

	ec.Disconnect()

	logger.Info("Ended.")
}
