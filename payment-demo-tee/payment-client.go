// You can edit this code!
// Click here and start typing.
package main

import (
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
	"io/ioutil"
	"os"
	"strconv"
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

type encryptedContent struct {
	Content []byte `json:content`
	Nonce   []byte `json:nonce` // used for decrypting amount
}

type payload struct {
	From   string           `json:from`
	To     string           `json:to`
	State  encryptedContent `json:state`
	Elf    encryptedContent `json:elf`
	Nonces [][]byte         `json:nonces` // used for encrypting PutPrivateData
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

func aesEncrypt(plaintext []byte) *encryptedContent {
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
	return &encryptedContent{ciphertext, nonce}
}

func getCiphertextOfData() (balance, x, elf *encryptedContent) {
	plaintextBalance := make([]byte, 16)
	binary.BigEndian.PutUint32(plaintextBalance, 100)
	balance = aesEncrypt(plaintextBalance)

	plaintextX := make([]byte, 16)
	binary.BigEndian.PutUint32(plaintextX, uint32(amount))
	x = aesEncrypt(plaintextX)

	plaintextElf, err := ioutil.ReadFile("./elf_payment.hex")
	if err != nil {
		panic(err.Error())
	}
	paddingCount := (len(plaintextElf) / 32 + 1) * 32 - len(plaintextElf) // elf/hex has to be integral multiples of 256 bits/32 bytes
	for i := 0; i < paddingCount; i++ {
		plaintextElf = append(plaintextElf, 0)
	}

	elf = aesEncrypt(plaintextElf)
	return
}

func decryptState(state *encryptedContent) int {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	plaintext, err := aesgcm.Open(nil, state.Nonce, state.Content, nil)
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

	balance, x, elf := getCiphertextOfData()
	clients[0].CreateAccount(0, balance)
	clients[0].CreateAccount(1, balance)
	txid, err := clients[0].Transfer(0, 1, x, elf)
	if err != nil {
		logger.Errorf("transfer from 0 to 1 failed. txid: %v, error: %v", txid, err.Error())
	} else {
		logger.Infof("transfer from 0 to 1 succeed. txid: %v", txid)
	}
	clients[0].GetState(0)
	clients[0].GetState(1)

	txid, err = clients[0].Transfer(0, 1, x, elf)
	if err != nil {
		logger.Errorf("transfer from 0 to 1 failed. txid: %v, error: %v", txid, err.Error())
	} else {
		logger.Infof("transfer from 0 to 1 succeed. txid: %v", txid)
	}
	clients[0].GetState(0)
	clients[0].GetState(1)

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
		balance := c.GetState(i)
		totalAmount += balance
	}
	return totalAmount
}

func (c *PaymentClient) CreateAccount(index int, amount *encryptedContent) error {
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
	state := encryptedContent{}
	json.Unmarshal(response.Payload, &state)

	balance := decryptState(&state)
	logger.Infof("%v : %d", index, balance)
	return balance
}

func (c *PaymentClient)  Transfer(from, to int, amount, elf *encryptedContent) (string, error) {
	// generate nonces for encrypting updated state.
	nonces := make([][]byte, 2)
	for i:=0; i < 2; i++ {
		nonce := make([]byte, 12)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			panic(err.Error())
		}
		nonces[i] = nonce
	}

	tmp := payload{From: strconv.Itoa(from), To: strconv.Itoa(to), State: *amount, Elf: *elf, Nonces:nonces}
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
