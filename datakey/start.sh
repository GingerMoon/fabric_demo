#dep init
go build
export TEE_FPGA_WORKERS=10 TEE_FPGA_SERVER_ADDR=192.168.0.102:50121
./datakey