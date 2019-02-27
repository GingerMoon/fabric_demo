package main

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/pkg/errors"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

const (
	AESKEY        = "AESKEY"
	ECDSAKEY      = "ECDSAKEY_PRI"
	ECDSAKEY_FROM = "ECDSAKEY_FROM"
	ECDSAKEY_TO   = "ECDSAKEY_TO"
	IV            = "IV"
	COLLECTION 	  = "collectionPayment"
)

var (
	logger = shim.NewLogger("payment_cc")
	iv     = make([]byte, 16)
)

// Paymentcc example simple Chaincode implementation
type Paymentcc struct {
	bccspInst bccsp.BCCSP
}

type Payload struct {
	From   string `json:from`
	To     string `json:to`
	Amount string `json:amount`
	Blob   [2]byte `json:blob`
}

func (a *Payload) ToBytes() ([]byte, error) {
	return json.Marshal(a)
}

func (a *Payload) FromBytes(d []byte) error {
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

func (t *Paymentcc) Init(stub shim.ChaincodeStubInterface) pb.Response {
	logger.Info("Init")
	return shim.Success(nil)
}

func (t *Paymentcc) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	// get arguments and transient
	f, args := stub.GetFunctionAndParameters()

	logger.Infof("function: %s", f)
	for i, arg := range args {
		logger.Infof("receives args[%d]: %s", i, arg)
	}

	switch f {
	case "create":
		return t.create(stub, args)
	case "query":
		return t.query(stub, args)
	case "transfer":
		return t.transfer(stub, args)
	default:
		return shim.Error(fmt.Sprintf("Unsupported function %s", f))
	}
}

// arg0 is the payload, payload.To is the state db key, which is also the public key.
func (t *Paymentcc) create(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	payload_str := args[0]

	var payload Payload
	payload.FromBytes([]byte(payload_str))

	if _, err := strconv.Atoi(payload.Amount); err != nil {
		return shim.Error("Expecting integer value for asset holding")
	}

	err := t.putBalance(stub, payload.To, payload.Amount)
	if err != nil {
		return shim.Error(fmt.Sprintf("put balance %s for %s failed, err %+v", args[1], args[0], err))
	}

	return shim.Success(nil)
}

// arg0 is the world state key
func (t *Paymentcc) query(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the person to query")
	}

	key := args[0]
	account, err := t.getAccountInfo(stub, key)
	cleartextValue, err := account.ToBytes()
	if err != nil {
		return shim.Error(fmt.Sprintf("getStateDecryptAndVerify failed, err %+v", err))
	}

	return shim.Success(cleartextValue)
}

func (t *Paymentcc) getAccountInfo (stub shim.ChaincodeStubInterface, key string) (*accountInfo, error) {
	accountInfobytes, err := stub.GetPrivateData(COLLECTION, key)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var account accountInfo
	account.FromBytes(accountInfobytes)

	return &account, nil
}

func (t *Paymentcc) getBalance (stub shim.ChaincodeStubInterface, key string) (int, error) {
	account, err := t.getAccountInfo(stub, key)
	if err != nil {
		return -1, errors.WithStack(errors.WithMessage(err, fmt.Sprintf("get account for %s failed.", key)))
	}

	balance, err := strconv.Atoi(account.Balance)
	if err != nil {
		return -1, errors.WithStack(err)
	}

	return balance, err
}

func (t *Paymentcc) putBalance (stub shim.ChaincodeStubInterface, key string, balance string) error {
	logger.Infof("put %s : %s", key, balance)

	// sign, then encrypt, then put state
	a := accountInfo{Balance: balance}
	payload, err := a.ToBytes()
	if err != nil {
		return errors.WithStack(err)
	}

	err = stub.PutPrivateData(COLLECTION, key, payload)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Transfer from A to B.
// arg0 is payload
func (t *Paymentcc) transfer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	payload_str := args[0]
	var payload Payload
	payload.FromBytes([]byte(payload_str))

	// get balance of A and B
	balanceA, err := t.getBalance(stub, payload.From)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get balance for account %s failed.", payload.From)).Error())
	}
	logger.Infof("before transfer, %s's balance is %d", payload.From, balanceA)

	balanceB, err := t.getBalance(stub, payload.To)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get balance for account %s failed.", payload.To)).Error())
	}
	logger.Infof("before transfer, %s's balance is %d", payload.To, balanceB)

	// check if A's balance is enough or not and if YES transfer (A-x, B+x)
	X, _ := strconv.Atoi(string(payload.Amount))
	logger.Infof("transfer v% from %s to %s", X, payload.From, payload.To)

	balanceA = balanceA - X
	if balanceA < 0 {
		return shim.Error(fmt.Sprintf("account %s has not enough balance (%d) to Transfer %d.", args[0], balanceA + X, X))
	}
	err = t.putBalance(stub, payload.From, strconv.Itoa(balanceA))
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("put balance for account %s failed.", args[0])).Error())
	}

	balanceB = balanceB + X
	t.putBalance(stub, payload.To, strconv.Itoa(balanceB))
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("put balance for account %s failed.", args[1])).Error())
	}

	fmt.Printf("balanceA = %d, balanceB = %d\n", balanceA, balanceB)
	return shim.Success(nil)
}

func main() {
	logger.SetLevel(shim.LogInfo)

	factory.InitFactories(nil)
	err := shim.Start(&Paymentcc{factory.GetDefault()})
	if err != nil {
		logger.Errorf("Error starting payment chaincode: %s", err)
	}
}
