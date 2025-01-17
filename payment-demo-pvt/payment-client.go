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

type payload struct {
	From   string `json:from`
	To     string `json:to`
	Amount int `json:amount`
}

func (a *payload) ToBytes() ([]byte, error) {
	return json.Marshal(a)
}

func (a *payload) FromBytes(d []byte) error {
	return json.Unmarshal(d, a)
}


type accountInfo struct {
	Balance int  `json: "balance"`
}

func (a *accountInfo) ToBytes() ([]byte, error) {
	return json.Marshal(a)
}

func (a *accountInfo) FromBytes(d []byte) error {
	return json.Unmarshal(d, a)
}

var (
	clientamount, accounts, amount = getEnvironment()
	elapsed4CreateAccounts = 0
	elapsed4Transfer = 0
	elapsed4Query = 0
)

func getEnvironment() (int, int, int) {
	val, ok := os.LookupEnv("CLIENT_AMOUNT")
	if !ok {
		logger.Fatalf("Please set environment variable CLIENT_AMOUNT")
	}
	clientamount, err := strconv.Atoi(val)
	if err != nil {
		logger.Fatalf("Illeagle environment variable CLIENT_AMOUNT: %s", val)
	}

	val, ok = os.LookupEnv("ACCOUNTS")
	if !ok {
		logger.Fatalf("Please set environment variable ACCOUNTS")
	}
	accounts, err := strconv.Atoi(val)
	if err != nil {
		logger.Fatalf("Illeagle environment variable ACCOUNTS: %s", val)
	}

	val, ok = os.LookupEnv("AMOUNT")
	if !ok {
		logger.Fatalf("Please set environment variable AMOUNT")
	}
	amount, err := strconv.Atoi(val)
	if err != nil {
		logger.Fatalf("Illeagle environment variable AMOUNT: %s", val)
	}

	return clientamount, accounts, amount
}

func Demo() error {

	logger.Info("initializing sdk...")
	configPath := "config-payment.yaml"
	sdk, err := fabsdk.New(config.FromFile(configPath))
	if err != nil {
		return errors.WithMessage(err, "Failed to create new SDK: %s")
	}
	defer sdk.Close()

	logger.Infof("Creating %d clients", clientamount)
	clients := make([]*PaymentClient, clientamount)
	for i := 0; i < clientamount; i++ {
		client, err := New(sdk)
		if err != nil {
			return errors.WithStack(err)
		}
		clients[i] = client
	}

	CreateAccounts(clients)

	logger.Infof("Before the transactions, the total amount of the network is %d", GetNetworkTotalAmount(clients))
	Transfer(clients)
	logger.Infof("After the transactions, the total amount of the network is %d", GetNetworkTotalAmount(clients))

	logger.Infof("Queries: %d, Elapsed time: %dms, QPS: %d", accounts, elapsed4Query, accounts*1000/elapsed4Query)
	logger.Infof("CreateAccounts: %d, Elapsed time: %dms, TPS: %d", accounts, elapsed4CreateAccounts, accounts*1000/elapsed4CreateAccounts)
	logger.Infof("Transfer: %d, Elapsed time: %dms, TPS: %d", accounts, elapsed4Transfer, accounts*1000/elapsed4Transfer)
	return nil
}

func CreateAccounts(clients []*PaymentClient) {
	var fense sync.WaitGroup
	start := time.Now()

	// crate accounts in the blockchain.
	for c, _ := range clients {
		fense.Add(1)
		go func(cc int) {
			defer fense.Done()
			for i := cc; i < accounts; i += len(clients) {
				clients[i%clientamount].CreateAccount(i, 100)
			}
		}(c)
	}
	fense.Wait()
	elapsed4CreateAccounts = int(time.Since(start) / time.Millisecond)
}

func GetNetworkTotalAmount(clients []*PaymentClient) int {
	var fense sync.WaitGroup
	start := time.Now()

	totalAmount := 0
	ch := make(chan int)

	fense.Add(1)
	go func() {
		defer fense.Done()
		for {
			balance, ok := <- ch
			if !ok {
				return
			} else {
				totalAmount += balance
			}
		}
	}()

	var w sync.WaitGroup
	for c, _ := range clients {
		w.Add(1)
		go func(cc int) {
			defer w.Done()
			for i := cc; i < accounts; i += len(clients) {
				accountinfoStr := clients[i%clientamount].GetState(i)
				var accountinfo accountInfo
				accountinfo.FromBytes([]byte(accountinfoStr))
				ch <- accountinfo.Balance
			}
		}(c)
	}
	w.Wait()
	close(ch)

	fense.Wait()
	elapsed4Query = int(time.Since(start) / time.Millisecond)
	return totalAmount
}

func Transfer(clients []*PaymentClient) {
	var fense sync.WaitGroup
	start := time.Now()

	// print failed tx message at last in a batch
	txErrCh := make(chan string, 100)
	fense.Add(1)
	go func () {
		defer fense.Done()
		l := list.New()
		for {
			errmsg, ok := <-txErrCh
			if ok {
				l.PushBack(errmsg)
			} else {
				break
			}
		}

		logger.Infof("----- the following transactions failed: -----")
		for e:= l.Front(); e != nil; e = e.Next() {
			logger.Infof("\n %v \n ", e.Value)
		}
	}()

	// simulate the transaction
	s1 := mrand.NewSource(time.Now().UnixNano())
	r1 := mrand.New(s1)

	var w sync.WaitGroup
	for c, _ := range clients {
		w.Add(1)
		go func(cc int) {
			defer w.Done()
			for i := cc; i < accounts; i += len(clients) {
				from := r1.Intn(clientamount)
				to := r1.Intn(clientamount)
				_, err := clients[i%clientamount].Transfer(from, to, amount)
				if err != nil {
					txErrCh <- err.Error()
				}
			}
		}(c)
	}
	w.Wait()
	close(txErrCh)

	fense.Wait()
	elapsed4Transfer = int(time.Since(start) / time.Millisecond)
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

func (c *PaymentClient) transfer() {

	// print failed tx message at last in a batch
	txErrCh := make(chan string, 100)
	defer close(txErrCh)
	go func () {
		l := list.New()
		for {
			errmsg := <-txErrCh
			if len(errmsg) != 0 {
				l.PushBack(errmsg)
			} else {
				break
			}
		}

		logger.Infof("----- the following transactions failed: -----")
		for e:= l.Front(); e != nil; e = e.Next() {
			logger.Infof("\n %v \n ", e.Value)
		}
	}()

	// simulate the transaction
	s1 := mrand.NewSource(time.Now().UnixNano())
	r1 := mrand.New(s1)
	var wg sync.WaitGroup
	for i := 0; i < clientamount*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			from := r1.Intn(clientamount)
			to := r1.Intn(clientamount)
			_, err := c.Transfer(from, to, amount)
			if err != nil {
				txErrCh <- err.Error()
			}
		}()
	}
	wg.Wait()
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

func (c *PaymentClient) CreateAccount(index, amount int) error {
	tmp := payload{From: "", To: strconv.Itoa(index), Amount: amount}
	payload, err := tmp.ToBytes()
	if err != nil {
		return errors.WithMessage(err, "CreateAccount failed (marshall payload).")
	}

	args := [][]byte{payload}

	_, err = c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "create", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		logger.Fatalf("Failed to create account: %s", err)
	}
	logger.Infof("created account: %v - %v", index, amount)
	return nil
}

func (c *PaymentClient) GetState(index int) string {
	args := [][]byte{[]byte(strconv.Itoa(index))}

	response, err := c.client.Query(
		channel.Request{ChaincodeID: ccID, Fcn: "query", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		logger.Fatalf("Failed to query funds: %s", err)
	}
	logger.Infof("%v : %v", index, string(response.Payload))
	return string(response.Payload)
}

func (c *PaymentClient) Transfer(from, to, amount int) (string, error) {
	tmp := payload{From: strconv.Itoa(from), To: strconv.Itoa(to), Amount: amount}
	payload, err := tmp.ToBytes()
	if err != nil {
		return "", errors.WithMessage(err, "Transfer failed (marshall payload).")
	}

	args := [][]byte{payload}

	response, err := c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "transfer", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return "",  errors.WithMessage(err, fmt.Sprintf("Transfer(%s) failed. from %d to %d. \n payload is %s.", response.TransactionID, from, to, payload))
	}
	logger.Infof("Transfer(%s) succeeded. from %d to %d. \n payload is %s.", response.TransactionID, from, to, payload)
	return string(response.TransactionID), nil
}
