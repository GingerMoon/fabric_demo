package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
	"os"
)

var (
	//logger = shim.NewLogger("payment_cc")
	logger = flogging.MustGetLogger("payment_cc")
)

type Payload struct {
	From   string `json:from`
	To     string `json:to`
	Amount uint32 `json:amount`
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

	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, payload.Amount)
	err := stub.PutState(payload.To, bs)
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
	balance, err := stub.GetState(key)
	if err != nil {
		return shim.Error(fmt.Sprintf("GetState %s failed. Err: %s", key, err.Error()))
	}

	return shim.Success(balance)
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

	// get balance of A and B
	balanceA, err := stub.GetState(payload.From)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get balance for account %s failed.", payload.From)).Error())
	}

	balanceB, err := stub.GetState(payload.To)
	if err != nil {
		return shim.Error(errors.WithMessage(err, fmt.Sprintf("get balance for account %s failed.", payload.To)).Error())
	}

	bsAmout := make([]byte, 4)
	binary.LittleEndian.PutUint32(bsAmout, payload.Amount)

	var teeArgs [][]byte
	teeArgs = append(teeArgs, []byte("paymentCCtee"))
	teeArgs = append(teeArgs, balanceA)
	teeArgs = append(teeArgs, balanceB)
	teeArgs = append(teeArgs, bsAmout)

	results, err := stub.TeeExecute(teeArgs)
	if err != nil {
		return shim.Error(fmt.Sprintf("Tee Execution failed! error: %s", err.Error()))
	}
	if len(results) != 2 {
		return shim.Error(fmt.Sprintf("Tee Execution returns incorrect response. %d", len(results)))
	}
	stub.PutState(payload.From, results[0])
	stub.PutState(payload.To, results[1])

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
