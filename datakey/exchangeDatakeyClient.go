package main

import (
	"context"
	pb "github.com/hyperledger/fabric/peer/datakey/protos"
	"google.golang.org/grpc"
	"os"
	"strconv"
	"time"
)

var (
	serverAddr = os.Getenv("TEE_FPGA_SERVER_ADDR")

	workers     []*worker
	taskPool      chan *task
)

type out struct {
	errCh chan <- error
}

type task struct {
	in  *pb.DataKeyArgs
	out *out
}

type worker struct {
	conn pb.TeeClient
	taskPool chan *task
}

func(w *worker) start() {
	for {
		task := <-w.taskPool
		// ensure that we don't have too many concurrent verify workers
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := w.conn.ExchangeDataKey(ctx, task.in)
			if err != nil {
				logger.Errorf("Execute() failed!  %v: ", err)
				task.out.errCh <- err
			} else {
				task.out.errCh <- nil
			}
		}()
	}
}

func init() {
	val := os.Getenv("TEE_FPGA_WORKERS")
	nWorkers, err := strconv.Atoi(val)
	if err != nil {
		logger.Warningf("TEE_FPGA_WORKERS %s is illegal! Now set it to 10.", val)
	}
	if nWorkers == 0 {
		nWorkers = 10
	}
	logger.Infof("nWorkers is: %d", nWorkers)
	taskPool = make(chan *task, nWorkers)
	workers = make([]*worker, nWorkers)
	for i := 0; i < len(workers); i++ {
		workers[i] = &worker{conn:createTeeClient(), taskPool:taskPool}
		go workers[i].start()
	}
}

func createTeeClient() pb.TeeClient {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	if serverAddr == "" {
		serverAddr = "peer0.org1.example.com:20000"
		logger.Infof("TEE_FPGA_SERVER_ADDR is not set. Now set it as default value: %s", serverAddr)
	}
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		logger.Fatalf("fail to dial: %v", err)
	}
	//defer conn.Close()
	return pb.NewTeeClient(conn)
}

func ExchangeDataKey(datakey, label []byte) error {
	errCh := make(chan error)
	taskPool <- &task{&pb.DataKeyArgs{Datakey:datakey, Label:label}, &out{errCh}}
	return <-errCh
}
