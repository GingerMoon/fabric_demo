#dep init
go build
# onl client in one goroutine
export CLIENT_AMOUNT=1000
export ACCOUNTS=1000
export AMOUNT=60
./payment-demo
