#dep init
go build

export CLIENT_AMOUNT=10 # not used in benchmark test
export ACCOUNTS=100 # not used in benchmark test
export AMOUNT=80

./payment-demo
