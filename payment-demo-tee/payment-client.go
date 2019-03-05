// You can edit this code!
// Click here and start typing.
package main

import (
	"container/list"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/pkg/errors"
	"io"
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

var logger = flogging.MustGetLogger("payment-demo-tee")

var key = []byte {
0xee, 0xbc, 0x1f, 0x57, 0x48, 0x7f, 0x51, 0x92, 0x1c, 0x04, 0x65, 0x66,
0x5f, 0x8a, 0xe6, 0xd1, 0x65, 0x8b, 0xb2, 0x6d, 0xe6, 0xf8, 0xa0, 0x69,
0xa3, 0x52, 0x02, 0x93, 0xa5, 0x72, 0x07, 0x8f,
}

type state struct {
	Amount []byte `json:amount`
	Nonce   []byte `json:nonce`
}

type payload struct {
	From   string `json:from`
	To     string `json:to`
	State  state `json:state`
}

func (a *payload) ToBytes() ([]byte, error) {
	return json.Marshal(a)
}

func (a *payload) FromBytes(d []byte) error {
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

func aesEncrypt(plaintext []byte) *state {
		block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	logger.Infof("plaintext is: %s, ciphertext is: %s", base64.StdEncoding.EncodeToString(plaintext), base64.StdEncoding.EncodeToString(ciphertext))
	return &state{ciphertext, nonce}
}

func getCiphertextOfData() (balance, x *state) {
	plaintextBalance := make([]byte, 4)
	binary.BigEndian.PutUint32(plaintextBalance, 100)
	balance = aesEncrypt(plaintextBalance)

	plaintextX := make([]byte, 4)
	binary.BigEndian.PutUint32(plaintextX, uint32(amount))
	x = aesEncrypt(plaintextX)
	return
}

func decryptState(state *state) int {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	plaintext, err := aesgcm.Open(nil, state.Nonce, state.Amount, nil)
	if err != nil {
		panic(err.Error())
	}
	return int(binary.BigEndian.Uint32(plaintext))
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

	balance, x := getCiphertextOfData()
	clients[0].CreateAccount(0, balance)
	clients[0].CreateAccount(1, balance)
	txid, err := clients[0].Transfer(0, 1, x)
	if err != nil {
		logger.Errorf("transfer from 0 to 1 failed. txid: %v, error: %v", txid, err.Error())
	} else {
		logger.Errorf("transfer from 0 to 1 succeed. txid: %v", txid)
	}
	clients[0].GetState(0)
	clients[0].GetState(1)

	//CreateAccounts(clients)
	//
	//logger.Infof("Before the transactions, the total amount of the network is %d", GetNetworkTotalAmount(clients))
	//Transfer(clients)
	//logger.Infof("After the transactions, the total amount of the network is %d", GetNetworkTotalAmount(clients))
	//
	//logger.Infof("Queries: %d, Elapsed time: %dms, QPS: %d", accounts, elapsed4Query, accounts*1000/elapsed4Query)
	//logger.Infof("CreateAccounts: %d, Elapsed time: %dms, TPS: %d", accounts, elapsed4CreateAccounts, accounts*1000/elapsed4CreateAccounts)
	//logger.Infof("Transfer: %d, Elapsed time: %dms, TPS: %d", accounts, elapsed4Transfer, accounts*1000/elapsed4Transfer)
	return nil
}

func CreateAccounts(clients []*PaymentClient) {
	balance, _ := getCiphertextOfData()

	var fense sync.WaitGroup
	start := time.Now()

	// crate accounts in the blockchain.
	for c, _ := range clients {
		fense.Add(1)
		go func(cc int) {
			defer fense.Done()
			for i := cc; i < accounts; i += len(clients) {
				clients[i%clientamount].CreateAccount(i, balance)
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
				balance := clients[i%clientamount].GetState(i)
				ch <- balance
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
	_, x := getCiphertextOfData()

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
				_, err := clients[i%clientamount].Transfer(from, to, x)
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
	_, x := getCiphertextOfData()

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
			_, err := c.Transfer(from, to, x)
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
		balance := c.GetState(i)
		totalAmount += balance
	}
	return totalAmount
}

func (c *PaymentClient) CreateAccount(index int, amount *state) error {
	tmp := payload{From: "", To: strconv.Itoa(index), State: *amount}
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

func (c *PaymentClient) GetState(index int) int {
	args := [][]byte{[]byte(strconv.Itoa(index))}

	response, err := c.client.Query(
		channel.Request{ChaincodeID: ccID, Fcn: "query", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		logger.Fatalf("Failed to query funds: %s", err)
	}
	state := state{}
	json.Unmarshal(response.Payload, &state)

	balance := decryptState(&state)
	logger.Infof("%v : %d", index, balance)
	return balance
}

func (c *PaymentClient)  Transfer(from, to int, amount *state) (string, error) {
	tmp := payload{From: strconv.Itoa(from), To: strconv.Itoa(to), State: *amount}
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
