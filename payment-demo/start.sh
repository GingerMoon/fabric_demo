cd /opt/gopath/src/github.com/hyperledger/fabric/peer/accelor-demo/payment-demo
#dep init
go build
export CLIENTAMOUNT=5
export AMOUNT=80
./accelor-demo
