package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	pbtee "github.com/hyperledger/fabric/protos/tee"
	"io"
	"os"
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
	Value []byte `json:value`
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

// TODO
// ended auction cannot be ended again.
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

	// TODO: check whether it's time to end auction.
	//end, err := time.Parse(TIMEFORMAT, auctionState.End)
	//if err != nil {
	//	return shim.Error(fmt.Sprintf("parse auciton end time failed. err: %s", err))
	//}
	//now := time.Now()
	//if now.Before(end) {
	//	return shim.Error(fmt.Sprintf("The auction hasm't been ended. Now: %s. Ended: %s", now.Format(TIMEFORMAT), end.Format(TIMEFORMAT)))
	//}

	// find the winner bid
	var winnerBid Bid
	for i, bidId := range auctionState.Bids {
		bidStateBytes, err := stub.GetPrivateData(COLLECTION, bidId)
		if err != nil {
			return shim.Error(fmt.Sprintf("get bidder state (%s) failed, err: %+v", bidId, err))
		}
		var bidState stateBid
		err = json.Unmarshal(bidStateBytes, &bidState)
		if err != nil {
			return shim.Error(fmt.Sprintf("Unmarshal bid (%s : %s) failed! err: %+v", bidId, bidStateBytes, err))
		}

		if i == 0 {
			winnerBid.Id = bidId
			winnerBid.State = bidState
			auctionState.Winner = bidId
			auctionState.Value = base64.StdEncoding.EncodeToString(bidState.Value)
			continue
		}

		// Tee execution
		var feed4decrytions []*pbtee.Feed4Decryption
		feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:[]byte(bidState.Value), Nonce:bidState.Nonce})
		feed4decrytions = append(feed4decrytions, &pbtee.Feed4Decryption{Ciphertext:[]byte(winnerBid.State.Value), Nonce:winnerBid.State.Nonce})

		results, err := stub.TeeExecute([]byte("compare"), nil, feed4decrytions, nil)
		if err != nil {
			return shim.Error(fmt.Sprintf("Tee Execution failed! error: %s", err.Error()))
		}
		if len(results.Plaintexts) != 1 {
			return shim.Error(fmt.Sprintf("Tee Execution returns incorrect response. %d", len(results.Plaintexts)))
		}

		// compare bidState and winnerBid
		if  string(results.Plaintexts[0]) == "1" { // bidState is bigger than winnerBid
			auctionState.Winner = bidId
			auctionState.Value = base64.StdEncoding.EncodeToString(bidState.Value)
			winnerBid.Id = bidId
			winnerBid.State = bidState
		} else if string(results.Plaintexts[0]) == "0" { // bidState equals winnerBid
			auctionState.Winner = "" // tie
		}
	}

	// update the auction state
	auctionStatebytes, _ = json.Marshal(auctionState)
	stub.PutState(auctionId, auctionStatebytes)

	return shim.Success([]byte(fmt.Sprintf("the winner bid id is %s, value: %s", auctionState.Winner, auctionState.Value)))
}

// TODO
// only ended auction can be quired.
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
	if len(args) != 4 {
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
	value := []byte(args[2])
	nonce := []byte(args[3])
	//value, err := base64.StdEncoding.DecodeString(args[2])
	//if err != nil {
	//	return shim.Error(fmt.Sprintf("args[2] need to be base64 encoded string. err: %+v", err))
	//}
	//nonce, err := base64.StdEncoding.DecodeString(args[3])
	//if err != nil {
	//	return shim.Error(fmt.Sprintf("args[3] need to be base64 encoded string. err: %+v", err))
	//}

	// TODO: value cannot be smaller than the StartingBid -- this should be calculated in tee. for now, we don't do this check for simplicity.
	//bidPrice, err := strconv.Atoi(value)
	//if err != nil {
	//	return shim.Error(fmt.Sprintf("Invalid bid value: %s. err %+v", value, err))
	//}
	//startingBid, err := strconv.Atoi(string(auctionState.Value))
	//if err != nil {
	//	return shim.Error(fmt.Sprintf("Invalid starting bid price: %s. err %+v", auctionState.Value, err))
	//}
	//if bidPrice <= startingBid {
	//	return shim.Error(fmt.Sprintf("The bid price (%d) must bigger than the starting bid price (%d).", bidPrice, startingBid))
	//}

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
		Nonce:nonce,
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