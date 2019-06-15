#dep init
go build
export balanceFrom=100
export balanceTo=100
export amount=80
export TEE_FPGA_SERVER_ADDR=192.168.0.102:50121
./payment-demo-tee
