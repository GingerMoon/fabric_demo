package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	"io"
	"os"
	"strconv"
	"time"
)

const (
	COLLECTION 	  = "collectionAuction"
	TIMEFORMAT	   = "2006-01-02 15:04:05.999999999 -0700 MST"
)

var (
	//logger = shim.NewLogger("auction_cc")
	logger = flogging.MustGetLogger("auction_cc")
)

type stateAuction struct {
	Winner string `json:winner` // if Winner is "", then there is no winner (maybe there are tie bids)
	Value string `json:value`
	Start string `json:start`
	End string `json:end`
	Bids []string `bids`
}

type Auction struct {
	Id   string `json:auctionId`
	State     stateAuction `json:state`
}

type stateBid struct {
	Cert string `json:cert`
	Value string `json:value`
	Nonce   []byte `json:nonce` // used for decrypting amount
}

type Bid struct {
	Id   string `json:id`
	State stateBid `json:state`
}

type Auctioncc struct {
}

func (t *Auctioncc) Init(stub shim.ChaincodeStubInterface) pb.Response {
	logger.Info("Init")
	return shim.Success(nil)
}

func (t *Auctioncc) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	// get arguments and transient
	f, args := stub.GetFunctionAndParameters()

	logger.Infof("function: %s", f)
	for i, arg := range args {
		logger.Infof("receives args[%d]: %s", i, arg)
	}

	switch f {
	case "create":
		return t.create(stub, args)
	case "end":
		return t.end(stub, args)
	case "query":
		return t.query(stub, args)
	case "bid":
		return t.bid(stub, args)
	default:
		return shim.Error(fmt.Sprintf("Unsupported function %s", f))
	}
}

func (t *Auctioncc) create(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// stub.GetCreator() must be org1
	// arguments are start time and end time.
	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	StartingBid := args[2]

	start, err := time.Parse(TIMEFORMAT, args[0])
	if err != nil {
		return shim.Error(fmt.Sprintf("time parse (%s) failed. err %+v", args[0], err))
	}

	end, err := time.Parse(TIMEFORMAT, args[1])
	if err != nil {
		return shim.Error(fmt.Sprintf("time parse (%s) failed. err %+v", args[1], err))
	}

	// auction id is generated randomly.
	auctionIdbytes := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, auctionIdbytes); err != nil {
		return shim.Error(fmt.Sprintf("Generating autionId failed! err %+v", err))
	}

	auctionId := base64.StdEncoding.EncodeToString(auctionIdbytes)
	exists, _ := stub.GetState(auctionId)
	if exists != nil {
		return shim.Error(fmt.Sprintf("Create aution(%s) failed! The auction has already existed.", auctionId))
	}

	// fill in the state and store it.
	var state stateAuction
	state.Start = start.Format(TIMEFORMAT)
	state.End = end.Format(TIMEFORMAT)
	state.Value = StartingBid

	statebytes, err := json.Marshal(state)
	if err != nil {
		return shim.Error(fmt.Sprintf("Marshal auction (%s) state (%s) failed, err %+v", auctionId, statebytes, err))
	}
	err = stub.PutState(auctionId, statebytes)
	if err != nil {
		return shim.Error(fmt.Sprintf("put auction (%s) state (%s) failed, err %+v", auctionId, statebytes, err))
	}

	return shim.Success([]byte(auctionId))
}

func (t *Auctioncc) end(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// stub.GetCreator() must be org1
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}
	auctionId := args[0]

	// get auction state
	auctionStatebytes, err := stub.GetState(auctionId)
	if err != nil {
		return shim.Error(fmt.Sprintf("Get aution(%s) failed! err: %+v", auctionId, err))
	}
	var auctionState stateAuction
	err = json.Unmarshal([]byte(auctionStatebytes), &auctionState)
	if err != nil {
		return shim.Error(fmt.Sprintf("Unmarshal aution(%s : %s) failed! err: %+v", auctionId, auctionStatebytes, err))
	}

	// find the winner bid
	var winnerBid Bid
	winnerBid.State.Value = "0"
	for _, bidId := range auctionState.Bids {
		bidStateBytes, err := stub.GetPrivateData(COLLECTION, bidId)
		if err != nil {
			return shim.Error(fmt.Sprintf("get bidder state (%s) failed, err: %+v", bidId, err))
		}
		var bidState stateBid
		err = json.Unmarshal(bidStateBytes, &bidState)
		if err != nil {
			return shim.Error(fmt.Sprintf("Unmarshal bid (%s : %s) failed! err: %+v", bidId, bidStateBytes, err))
		}

		bidValue, err := strconv.Atoi(string(bidState.Value))
		if err != nil {
			return shim.Error(fmt.Sprintf("bid (%s : %s) - Atoi bidState.value failed! err: %+v", bidId, bidStateBytes, err))
		}
		winnerValue, err := strconv.Atoi(string(winnerBid.State.Value))
		if err != nil {
			return shim.Error(fmt.Sprintf("bid (%s : %s) - Atoi winnerValue.value failed! err: %+v", bidId, winnerValue, err))
		}

		if bidValue > winnerValue {
			logger.Infof("bidValue: %s, winnerValue: %d", string(bidState.Value), winnerValue)
			auctionState.Winner = bidId
			auctionState.Value = bidState.Value
			winnerBid.Id = bidId
			winnerBid.State = bidState
		} else if bidValue == winnerValue {
			logger.Infof("bidValue: %d, winnerValue: %d", bidValue, winnerValue)
			auctionState.Winner = "" // tie
		}
	}

	// update the auction state
	auctionStatebytes, _ = json.Marshal(auctionState)
	stub.PutState(auctionId, auctionStatebytes)

	return shim.Success([]byte(fmt.Sprintf("the winner bid id is %s, value: %s", auctionState.Winner, auctionState.Value)))
}

func (t *Auctioncc) query(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the auction to query")
	}

	auctionId := args[0]
	stateBytes, err := stub.GetState(auctionId)
	if err != nil {
		return shim.Error(fmt.Sprintf("GetState %s failed. Err: %s", auctionId, err.Error()))
	}

	return shim.Success(stateBytes)
}

func (t *Auctioncc) bid(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	// get cert and value
	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	// check the bid time
	auctionId := args[0]
	auctionStateBytes, err := stub.GetState(auctionId)
	if err != nil {
		return shim.Error(fmt.Sprintf("Get auction (%s) failed. err %+v", auctionId, err))
	}
	var auctionState stateAuction
	err = json.Unmarshal(auctionStateBytes, &auctionState)
	if err != nil {
		return shim.Error(fmt.Sprintf("unmarshal auciton (%s : %s) failed. err: %s", auctionId, auctionStateBytes, err))
	}
	start, err := time.Parse(TIMEFORMAT, auctionState.Start)
	if err != nil {
		return shim.Error(fmt.Sprintf("parse auciton (%s : %s) start time failed. err: %s", auctionId, auctionStateBytes, err))
	}
	end, err := time.Parse(TIMEFORMAT, auctionState.End)
	if err != nil {
		return shim.Error(fmt.Sprintf("parse auciton (%s : %s) end time failed. err: %s", auctionId, auctionStateBytes, err))
	}
	now := time.Now()
	if now.Before(start) {
		return shim.Error(fmt.Sprintf("The auction (%s : %s) hasn't started. Now: %s. Started: %s",
			auctionId, auctionStateBytes, now.Format(TIMEFORMAT), start.Format(TIMEFORMAT)))
	}
	if now.After(end) {
		return shim.Error(fmt.Sprintf("The auction (%s : %s) has already ended. Now: %s. Ended: %s",
			auctionId, auctionStateBytes, now.Format(TIMEFORMAT), end.Format(TIMEFORMAT)))
	}

	// get cert and bid value
	cert := args[1]
	value := args[2]

	// value cannot be smaller than StartingBid
	bidPrice, err := strconv.Atoi(value)
	if err != nil {
		return shim.Error(fmt.Sprintf("Invalid bid value: %s. err %+v", value, err))
	}
	startingBid, err := strconv.Atoi(string(auctionState.Value))
	if err != nil {
		return shim.Error(fmt.Sprintf("Invalid starting bid price: %s. err %+v", auctionState.Value, err))
	}
	if bidPrice <= startingBid {
		return shim.Error(fmt.Sprintf("The bid price (%d) must bigger than the starting bid price (%d).", bidPrice, startingBid))
	}

	// stub.GetCreator() must equal to cert, not completed because every time fabric restart, certs changed.

	// bid id is generated randomly.
	bidIdbytes := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, bidIdbytes); err != nil {
		return shim.Error(fmt.Sprintf("Generating bidIdbytes failed! err %+v", err))
	}

	bidId := base64.StdEncoding.EncodeToString(bidIdbytes)
	bidState := stateBid {
		Cert:cert,
		Value:value,
	}

	// put bid state
	bidStateBytes, err := json.Marshal(bidState)
	if err != nil {
		return shim.Error(fmt.Sprintf("Marshal bid (%s) state failed! err %+v", bidId, err))
	}
	stub.PutPrivateData(COLLECTION, bidId, bidStateBytes)

	// update auction
	auctionState.Bids = append(auctionState.Bids, bidId)
	auctionStateBytes, _ = json.Marshal(auctionState)
	stub.PutState(auctionId, auctionStateBytes)

	return shim.Success([]byte(bidId))
}

func main() {
	flogging.Init(flogging.Config{
		Writer:  os.Stderr,
		LogSpec: "DEBUG",
	})

	err := shim.Start(&Auctioncc{})
	if err != nil {
		logger.Errorf("Error starting payment chaincode: %s", err)
	}
}