#dep init
go build

export CLIENT_AMOUNT=10 # not used in benchmark test
export ACCOUNTS=100000 # not used in benchmark test
export AMOUNT=80

for (( ; ; ))
do
    ./payment-demo
    sleep 10
done
