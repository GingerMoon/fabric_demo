package main

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	pbtee "github.com/hyperledger/fabric/protos/tee"
	"github.com/pkg/errors"
	"os"
)

const (
	COLLECTION 	  = "collectionPayment"
)

var (
	//logger = shim.NewLogger("payment_cc")
	logger = flogging.MustGetLogger("payment_cc")
)

type encryptedContent struct {
	Content []byte `json:content`
	Nonce   []byte `json:nonce` // used for decrypting amount
}

type Payload struct {
	From   string `json:from`
	To     string `json:to`
	State  encryptedContent `json:state`
	Elf    encryptedContent `json:elf`
	Nonces [][]byte `json:nonces` // used for encrypting PutPrivateData
}

func (a *Payload) ToBytes() ([]byte, error) {
	return json.Marshal(a)
}

func (a *Payload) FromBytes(d []byte) error {
	return json.Unmarshal(d, a)
}

// Paymentcc example simple Chaincode implementation
type Paymentcc struct {
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

	// input parameters check
	if payload.From != "" {
		return shim.Error(fmt.Sprintf("Create account(%s) failed! The [from(%s)] account must be empty.", payload.To, payload.From))
	}
	if payload.To != "0" && payload.To != "1" {
		return shim.Error(fmt.Sprintf("Create account(%s) failed! The account can only be 0(org1) or 1(org2).", payload.To))
	}
	stateBytes, _ := stub.GetPrivateData(COLLECTION, payload.To)
	if stateBytes != nil {
		return shim.Error(fmt.Sprintf("Create account(%s) failed! The account has already exists!", payload.To))
	}

	// PutPrivateData
	state, _ := json.Marshal(payload.State)
	err := stub.PutPrivateData(COLLECTION, payload.To, state)
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
	stateBytes, err := stub.GetPrivateData(COLLECTION, key)
	if err != nil {
		return shim.Error(fmt.Sprintf("GetState %s failed. Err: %s", key, err.Error()))
	}

	return shim.Success(stateBytes)
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

	// input parameters check
	if payload.From != "0" && payload.From != "1" {
		return shim.Error(fmt.Sprintf("Transfer from account(%s) to account(%s) failed! The account can only be 0(org1) or 1(org2).", payload.From, payload.To))
	}
	if payload.To != "0" && payload.To != "1" {
		return shim.Error(fmt.Sprintf("Transfer from account(%s) to account(%s) failed! The account can only be 0(org1) or 1(org2).", payload.From, payload.To))
	}
	// creator check hasn't be finished because every time the fabric is restarted, the public key/certification is changed.
	//if payload.From == "0" {
	//	stub.GetCreator() must be 0
	//} else if payload.From == "1" {
	//	stub.GetCreator() must be 1
	//}

	if payload.From == payload.To {
		return shim.Success(nil)
	}

	// get private data
	stateAbytes, err := stub.GetPrivateData(COLLECTION, payload.From)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get state for account %s failed.", payload.From)).Error())
	}
	stateA := encryptedContent{}
	err = json.Unmarshal(stateAbytes, &stateA)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("unmarshall state for account %s failed.", payload.From)).Error())
	}

	stateBbytes, err := stub.GetPrivateData(COLLECTION, payload.To)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get balance for account %s failed.", payload.To)).Error())
	}
	stateB := encryptedContent{}
	err = json.Unmarshal(stateBbytes, &stateB)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("unmarshall state for account %s failed.", payload.From)).Error())
	}

	// Tee execution
	var feed4decrytions []*pbtee.Feed4Decryption
	feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:payload.Elf.Content, Nonce:payload.Elf.Nonce})
	feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:stateA.Content, Nonce:stateA.Nonce})
	feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:stateB.Content, Nonce:stateB.Nonce})
	feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:payload.State.Content, Nonce:payload.State.Nonce})

	results, err := stub.TeeExecute([]byte("paymentCCtee"), nil, feed4decrytions, payload.Nonces)
	if err != nil {
		return shim.Error(fmt.Sprintf("Tee Execution failed! error: %s", err.Error()))
	}
	if len(results.Feed4Decryptions) != 2 {
		return shim.Error(fmt.Sprintf("Tee Execution returns incorrect response. results.Feed4Decryptions is: %d", len(results.Feed4Decryptions)))
	}

	// update state db
	stateAbytes, err = json.Marshal(encryptedContent{Content:results.Feed4Decryptions[0].Ciphertext, Nonce:results.Feed4Decryptions[0].Nonce})
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("marshal Tee Execution results.Feed4Decryptions[0] failed")).Error())
	}
	stub.PutPrivateData(COLLECTION, payload.From, stateAbytes)

	stateBbytes, err = json.Marshal(encryptedContent{Content:results.Feed4Decryptions[1].Ciphertext, Nonce:results.Feed4Decryptions[1].Nonce})
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("marshal Tee Execution results.Feed4Decryptions[1] failed")).Error())
	}
	stub.PutPrivateData(COLLECTION, payload.To, stateBbytes)

	return shim.Success(nil)
}

func main() {
	flogging.Init(flogging.Config{
		Writer:  os.Stderr,
		LogSpec: "DEBUG",
	})

	err := shim.Start(&Paymentcc{})
	if err != nil {
		logger.Errorf("Error starting payment chaincode: %s", err)
	}
}