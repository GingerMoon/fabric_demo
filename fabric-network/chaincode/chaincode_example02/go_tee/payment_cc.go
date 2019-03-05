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

var (
	//logger = shim.NewLogger("payment_cc")
	logger = flogging.MustGetLogger("payment_cc")
)

type state struct {
	Amount []byte `json:amount`
	Nonce   []byte `json:nonce`
}

type Payload struct {
	From   string `json:from`
	To     string `json:to`
	State  state `json:state`
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

	state, _ := json.Marshal(payload.State)
	err := stub.PutState(payload.To, state)
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
	stateBytes, err := stub.GetState(key)
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

	if payload.From == payload.To {
		return shim.Success(nil)
	}

	// get state of A and B
	stateAbytes, err := stub.GetState(payload.From)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get state for account %s failed.", payload.From)).Error())
	}
	stateA := state{}
	err = json.Unmarshal(stateAbytes, &stateA)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("unmarshall state for account %s failed.", payload.From)).Error())
	}

	stateBbytes, err := stub.GetState(payload.To)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get balance for account %s failed.", payload.To)).Error())
	}
	stateB := state{}
	err = json.Unmarshal(stateBbytes, &stateB)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("unmarshall state for account %s failed.", payload.From)).Error())
	}

	// Tee execution
	var feed4decrytions []*pbtee.Feed4Decryption
	feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:stateA.Amount, Nonce:stateA.Nonce})
	feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:stateB.Amount, Nonce:stateB.Nonce})
	feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:payload.State.Amount, Nonce:payload.State.Nonce})

	results, err := stub.TeeExecute([]byte("paymentCCtee"), nil, feed4decrytions)
	if err != nil {
		return shim.Error(fmt.Sprintf("Tee Execution failed! error: %s", err.Error()))
	}
	if len(results.Feed4Decryptions) != 2 {
		return shim.Error(fmt.Sprintf("Tee Execution returns incorrect response. %d", len(results.Feed4Decryptions)))
	}

	// update state db
	stateAbytes, err = json.Marshal(results.Feed4Decryptions[0])
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("marshal Tee Execution results.Feed4Decryptions[0] failed")).Error())
	}
	stub.PutState(payload.From, stateAbytes)

	stateBbytes, err = json.Marshal(results.Feed4Decryptions[1])
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("marshal Tee Execution results.Feed4Decryptions[1] failed")).Error())
	}
	stub.PutState(payload.To, stateBbytes)

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
