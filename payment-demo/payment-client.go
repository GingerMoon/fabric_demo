// You can edit this code!
// Click here and start typing.
package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/pkg/errors"
	mrand "math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	channelID      = "mychannel"
	ccID           = "mycc"
	org1Name       = "Org1"
	org2Name       = "Org2"
	orgAdmin       = "Admin"
	ordererOrgName = "OrdererOrg"
	AESKEY         = "AESKEY"
)

var logger = flogging.MustGetLogger("payment-demo")

type statistics struct {
	// duration in ms
	querElapsedTime time.Duration
	tx4createElapsedTime time.Duration
	tx4transferElapsedTime time.Duration

	queries time.Duration
	txs4create time.Duration
	txs4transfer time.Duration

	transferTpsChan chan time.Duration
}

func (s8s *statistics) print() {

	logger.Infof("total query elapsed time: %dms. Queries: %d. QPS: %d",
		s8s.querElapsedTime, s8s.queries, s8s.queries*1000/s8s.querElapsedTime)

	logger.Infof("total tx(create) elapsed time: %ds. Txs(create): %d. TPS: %d",
		s8s.tx4createElapsedTime, s8s.txs4create, s8s.txs4create/s8s.tx4createElapsedTime)

	logger.Infof("total tx(transfer) elapsed time: %ds. Txs(transfer): %d. TPS: %d",
		s8s.tx4transferElapsedTime, s8s.txs4transfer, s8s.txs4transfer/s8s.tx4transferElapsedTime)
}

var s8s = &statistics {transferTpsChan:make(chan time.Duration)}

type payload struct {
	From   string `json:from`
	To     string `json:to`
	Amount string `json:amount`
	Blob   [2]byte `json:blob` // grpc limit & sha256
}

func (a *payload) ToBytes() ([]byte, error) {
	return json.Marshal(a)
}

func (a *payload) FromBytes(d []byte) error {
	return json.Unmarshal(d, a)
}


type accountInfo struct {
	Balance string  `json: "balance"`
	Blob    [2]byte `json: "blob"` // 1G exceeds the limitation of gRPC
}

func (a *accountInfo) ToBytes() ([]byte, error) {
	return json.Marshal(a)
}

func (a *accountInfo) FromBytes(d []byte) error {
	return json.Unmarshal(d, a)
}

var (
	clientamount, amount = getEnvironment()
)

func getEnvironment() (int, string) {
	val, ok := os.LookupEnv("CLIENTAMOUNT")
	if !ok {
		logger.Fatalf("Please set environment variable CLIENTAMOUNT")
	}
	clientamount, err := strconv.Atoi(val)
	if err != nil {
		logger.Fatalf("Illeagle environment variable CLIENTAMOUNT: %s", val)
	}

	amount, ok := os.LookupEnv("AMOUNT")
	if !ok {
		logger.Fatalf("Please set environment variable CLIENTAMOUNT")
	}
	return clientamount, amount
}

func Demo() error {
	// create sdk
	configPath := "config-payment.yaml"
	sdk, err := fabsdk.New(config.FromFile(configPath))
	if err != nil {
		return errors.WithMessage(err, "Failed to create new SDK: %s")
	}
	defer sdk.Close()
	// create client
	client, err := New(sdk)
	if err != nil {
		return errors.WithStack(err)
	}

	// crate accounts in the blockchain.
	for i := 0; i < clientamount; i++ {
		client.CreateAccount(i, "100")
	}

	// store error msg in channel and at last print them all in a batch
	txErrorCh := make(chan string)
	go func () {
		l := list.New()
		for {
			errmsg := <-txErrorCh
			if len(errmsg) == 0 {
				break
			}
			l.PushBack(errmsg)
		}

		logger.Infof("----- the following transactions failed: -----")
		for e:= l.Front(); e != nil; e = e.Next() {
			logger.Infof("\n %v \n ", e.Value)
		}
	}()

	logger.Infof("Before the transactions, the total amount of the network is %d", client.GetNetworkTotalAmount())

	// simulate the transaction
	s1 := mrand.NewSource(time.Now().UnixNano())
	r1 := mrand.New(s1)
	var wg sync.WaitGroup

	go func() {
		for {
			elapsed := <- s8s.transferTpsChan
			if elapsed <= 0 {
				break
			}
			s8s.tx4transferElapsedTime += elapsed
			s8s.txs4transfer++
		}
	}()

	for i := 0; i < clientamount*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			from := r1.Intn(clientamount)
			to := r1.Intn(clientamount)
			_, err := client.Transfer(from, to, amount)
			if err != nil {
				txErrorCh <- err.Error()
			}
		}()
	}
	wg.Wait()
	close(txErrorCh)
	close(s8s.transferTpsChan)
	logger.Infof("After the transactions, the total amount of the network is %d", client.GetNetworkTotalAmount())
	s8s.print()
	return nil
}

type PaymentClient struct {
	client *channel.Client
}

func New(sdk *fabsdk.FabricSDK) (*PaymentClient, error) {
	//prepare channel client context using client context
	clientChannelContext := sdk.ChannelContext(channelID, fabsdk.WithUser("User1"))
	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to create new channel client: %s")
	}
	return &PaymentClient{client}, nil
}

func (c *PaymentClient) GetNetworkTotalAmount() int {
	totalAmount := 0
	for i := 0; i < clientamount; i++ {
		accountinfoStr := c.GetState(i)
		var accountinfo accountInfo
		accountinfo.FromBytes([]byte(accountinfoStr))
		balance, _ := strconv.Atoi(string(accountinfo.Balance))
		totalAmount += balance
	}
	return totalAmount
}

func (c *PaymentClient) CreateAccount(index int, amount string) error {
	tmp := payload{From: "", To: strconv.Itoa(index), Amount: amount}
	payload, err := tmp.ToBytes()
	if err != nil {
		return errors.WithMessage(err, "CreateAccount failed (marshall payload).")
	}

	args := [][]byte{payload}

	start := time.Now()
	_, err = c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "create", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))
	s8s.tx4createElapsedTime += time.Since(start) / time.Second
	s8s.txs4create++

	if err != nil {
		logger.Fatalf("Failed to create account: %s", err)
	}
	logger.Infof("created account: %v - %v", index, amount)
	return nil
}

func (c *PaymentClient) GetState(index int) string {
	args := [][]byte{[]byte(strconv.Itoa(index))}

	start := time.Now()
	response, err := c.client.Query(
		channel.Request{ChaincodeID: ccID, Fcn: "query", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))
	s8s.querElapsedTime += time.Since(start) / time.Millisecond
	s8s.queries++

	if err != nil {
		logger.Fatalf("Failed to query funds: %s", err)
	}
	logger.Infof("%v : %v", index, string(response.Payload))
	return string(response.Payload)
}

func (c *PaymentClient) Transfer(from, to int, amount string) (string, error) {
	tmp := payload{From: strconv.Itoa(from), To: strconv.Itoa(to), Amount: amount}
	payload, err := tmp.ToBytes()
	if err != nil {
		return "", errors.WithMessage(err, "Transfer failed (marshall payload).")
	}

	args := [][]byte{payload}

	start := time.Now()
	response, err := c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "transfer", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))
	s8s.transferTpsChan <- time.Since(start) / time.Second

	if err != nil {
		return "",  errors.WithMessage(err, fmt.Sprintf("Transfer(%s) failed. from %d to %d. \n payload is %s.", response.TransactionID, from, to, payload))
	}
	logger.Infof("Transfer(%s) succeeded. from %d to %d. \n payload is %s.", response.TransactionID, from, to, payload)
	return string(response.TransactionID), nil
}
