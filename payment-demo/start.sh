#dep init
go build

export CLIENT_AMOUNT=2 # not used in benchmark test
export ACCOUNTS=2 # not used in benchmark test
export AMOUNT=80

./payment-demo
