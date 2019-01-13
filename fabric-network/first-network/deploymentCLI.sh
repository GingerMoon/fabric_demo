export ORG1_CONTAINER=
export ORG2_CONTAINER=
export ORDER_CONTAINER=

# Create channel -- mychannel cannot work here due to the fabric bug -- it says mychannel already exists. [it seems that the channel created in previous tests still exists]
export CHANNEL_NAME=mychannel
#non-TLS
peer channel create -o $ORDER_CONTAINER:7050 -c $CHANNEL_NAME -f ./channel-artifacts/channel.tx

# Join Channel
export CORE_PEER_ADDRESS=$ORG1_CONTAINER:7051
peer channel join -b mychannel.block

export CORE_PEER_ADDRESS=$ORG2_CONTAINER:7051
CORE_PEER_MSPCONFIGPATH=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp CORE_PEER_LOCALMSPID="Org2MSP" CORE_PEER_TLS_ROOTCERT_FILE=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt peer channel join -b mychannel.block

# Install Chaincode
export CORE_PEER_ADDRESS=$ORG1_CONTAINER:7051
peer chaincode install -n mycc -v 1.0 -p github.com/chaincode/chaincode_example02/go/ 

export CORE_PEER_ADDRESS=$ORG2_CONTAINER:7051
CORE_PEER_MSPCONFIGPATH=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp CORE_PEER_LOCALMSPID="Org2MSP" CORE_PEER_TLS_ROOTCERT_FILE=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt peer chaincode install -n mycc -v 1.0 -p github.com/chaincode/chaincode_example02/go/ 