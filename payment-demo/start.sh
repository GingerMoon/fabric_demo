#dep init
go build
# one client in one goroutine. If client amoutn is less than accounts, the tps of CreateAccount can be low.
export CLIENT_AMOUNT=80
export ACCOUNTS=1000
export AMOUNT=60
./payment-demo
