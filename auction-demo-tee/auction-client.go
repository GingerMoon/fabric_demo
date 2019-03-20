// You can edit this code!
// Click here and start typing.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/pkg/errors"
	"io"
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

type payloadBid struct {
	AuctionId string `json:auction_id`
	Value []byte `json:value`
	Nonce []byte `json:nonce`
	Cert []byte	 `json:cert`
}

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

type AES struct {
	Key []byte
}

func NewAES() *AES{
	return &AES {
		Key:[]byte {
			0xee, 0xbc, 0x1f, 0x57, 0x48, 0x7f, 0x51, 0x92, 0x1c, 0x04, 0x65, 0x66,
			0x5f, 0x8a, 0xe6, 0xd1, 0x65, 0x8b, 0xb2, 0x6d, 0xe6, 0xf8, 0xa0, 0x69,
			0xa3, 0x52, 0x02, 0x93, 0xa5, 0x72, 0x07, 0x8f,
		},
	}
}

func (a *AES) Encrypt(plaintext []byte) (ciphertext, nonce []byte) {
	block, err := aes.NewCipher(a.Key)
	if err != nil {
		panic(err.Error())
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce = make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	ciphertext = aesgcm.Seal(nil, nonce, plaintext, nil)
	logger.Infof("plaintext is: %s, ciphertext is: %s", base64.StdEncoding.EncodeToString(plaintext), base64.StdEncoding.EncodeToString(ciphertext))
	return
}

func (a *AES) Decrypt(ciphertext, nonce []byte) (plaintext []byte) {
	block, err := aes.NewCipher(a.Key)
	if err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	plaintext, err = aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}
	return
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

	//// start an auction
	//start := time.Now()
	//end := start.Add(time.Hour*1)
	//auctionId, txId, err := client.CreateAuction(start.Format(TIMEFORMAT), end.Format(TIMEFORMAT), "5")
	//if err != nil {
	//	logger.Errorf("Created auction failed! txId: %s. error: %s", txId, err)
	//	return err
	//}
	//logger.Infof("Created auction successfully. auctionId: %s. TxId: %s", auctionId, txId)

	//auctionId := "7ymLSRrZpIwMRomh"

	//// bid
	//a := NewAES()
	//cert := "Org2MSP"
	//ciphertext, nonce := a.Encrypt([]byte("40"))
	//bidId, txId, err := client.Bid(&payloadBid{AuctionId:auctionId, Value:ciphertext, Nonce:nonce, Cert:[]byte(cert)})
	//if err != nil {
	//	logger.Errorf("%s bided failed. TxId: %s. err: %s", cert, txId, err)
	//	return err
	//}
	//logger.Infof("%s bided successfully. BidId: %s. TxId: %s", cert, bidId, txId)

	//// end auction
	//result, txId, err := client.EndAuction(auctionId)
	//if err != nil {
	//	logger.Errorf("Ended auction failed. auctionId: %s. TxId: %s. err: %s", auctionId, txId, err)
	//	return err
	//}
	//logger.Infof("Ended auction successfully. auction: %s. TxId: %s",result, txId)

	//// query auction
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

func (c *AuctionClient)  Bid(bid *payloadBid) (bidId, txId string, err error) {
	args := [][]byte{[]byte(bid.AuctionId), bid.Cert, bid.Value, bid.Nonce}

	response, err := c.client.Execute(
		channel.Request{ChaincodeID: ccID, Fcn: "bid", Args: args},
		channel.WithRetry(retry.DefaultChannelOpts))

	if err != nil {
		return "", string(response.TransactionID),  err
	}
	return string(response.Payload), string(response.TransactionID), nil
}
