// You can edit this code!
// Click here and start typing.
package main

import (
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/pkg/errors"
	"time"
)

const (
	channelID      = "mychannel"
	ccID           = "mycc"
	org1Name       = "Org1"
	org2Name       = "Org2"
	org3Name       = "Org3"
	orgAdmin       = "Admin"
	ordererOrgName = "OrdererOrg"
	AESKEY         = "AESKEY"
	TIMEFORMAT	   = "2006-01-02 15:04:05.999999999 -0700 MST"
)

var logger = flogging.MustGetLogger("auction-demo-tee")

type stateAuction struct {
	Winner string `json:winner` // if Winner is "", then there is no winner (maybe there are tie bids)
	Value string `json:value`
	Start time.Time `json:start`
	End time.Time `json:end`
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

func Demo() error {

	logger.Info("initializing sdk...")
	configPath := "config-auction.yaml"
	sdk, err := fabsdk.New(config.FromFile(configPath))
	if err != nil {
		return errors.WithMessage(err, "Failed to create new SDK: %s")
	}
	defer sdk.Close()

	client, err := New(sdk)
	if err != nil {
		return errors.WithStack(err)
	}

	// start an auction
	start := time.Now()
	end := start.Add(time.Hour*1)
	auctionId, txId, err := client.CreateAuction(start.Format(TIMEFORMAT), end.Format(TIMEFORMAT), "5")
	if err != nil {
		logger.Errorf("Created auction failed! txId: %s. error: %s", txId, err)
		return err
	}
	logger.Infof("Created auction successfully. auctionId: %s. TxId: %s", auctionId, txId)

	//auctionId := "6vrlM1enG+4AnbRE"

	// bid 2
	bidId, txId, err := client.Bid(auctionId, "20", []byte("Org2MSP"))
	if err != nil {
		logger.Errorf("org2 bided failed. TxId: %s. err: %s", txId, err)
		return err
	}
	logger.Infof("org2 bided successfully. BidId: %s. TxId: %s", bidId, txId)

	// bid 3
	//bidId, txId, err = client.Bid(auctionId, "20", []byte("Org3MSP"))
	//if err != nil {
	//	logger.Errorf("org3 bided failed. TxId: %s. err: %s", txId, err)
	//	return err
	//}
	//logger.Infof("org3 bided successfully. BidId: %s. TxId: %s", bidId, txId)

	// end auction
	//result, txId, err := client.EndAuction(auctionId)
	//if err != nil {
	//	logger.Errorf("Ended auction failed. auctionId: %s. TxId: %s. err: %s", auctionId, txId, err)
	//	return err
	//}
	//logger.Infof("Ended auction successfully. auction: %s. TxId: %s",result, txId)

	// query auction
	//auction, err := client.QueryAuction(auctionId)
	//if err != nil {
	//	logger.Errorf("Query auction failed. auctionId: %s. err: %s", auctionId, err)
	//	return err
	//}
	//logger.Infof("Query auction successfully. auction: %s - %s.",auctionId, auction)
	return nil
}

type AuctionClient struct {
	client *channel.Client
}

func New(sdk *fabsdk.FabricSDK) (*AuctionClient, error) {
	//prepare channel client context using client context
	clientChannelContext := sdk.ChannelContext(channelID, fabsdk.WithUser("User1"))
	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to create new channel client: %s")
	}
	return &AuctionClient{client}, nil
}

func (c *AuctionClient) CreateAuction(start, end, StartingBid string) (auction, txId string, err error) {
	args := [][]byte{[]byte(start), []byte(end), []byte(StartingBid)}

	response, err := c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "create", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return
	}

	auction = string(response.Payload)
	txId = string(response.TransactionID)
	return
}

func (c *AuctionClient) EndAuction(auctionId string) (auction, txId string, err error) {

	args := make([][]byte, 1)
	args[0] = []byte(auctionId)

	response, err := c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "end", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return
	}

	auction = string(response.Payload)
	txId = string(response.TransactionID)
	return
}

func (c *AuctionClient) QueryAuction(auctionId string) (auction string, err error) {

	args := make([][]byte, 1)
	args[0] = []byte(auctionId)

	response, err := c.client.Query(
		channel.Request{ChaincodeID: ccID, Fcn: "query", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return
	}

	auction = string(response.Payload)
	return
}

func (c *AuctionClient)  Bid(auctionId, value string, cert []byte) (bidId, txId string, err error) {
	args := [][]byte{[]byte(auctionId), cert, []byte(value)}

	response, err := c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "bid", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return "", string(response.TransactionID),  err
	}
	return string(response.Payload), string(response.TransactionID), nil
}
