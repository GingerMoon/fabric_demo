package tee

import (
	"context"
	"github.com/hyperledger/fabric/common/flogging"
	pb "github.com/hyperledger/fabric/protos/tee"
	"google.golang.org/grpc"
	"os"
	"strconv"
	"time"
)

var (
	logger = flogging.MustGetLogger("tee")
	serverAddr = os.Getenv("TEE_FPGA_SERVER_ADDR")

	workers     []*worker
	taskPool      chan *task
)

type out struct {
	respCh chan <- *pb.PlainCiphertexts
	errCh chan <- error
}

type task struct {
	in  *pb.TeeArgs
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
			response, err := w.conn.Execute(ctx, task.in)
			if err != nil {
				logger.Errorf("Execute() failed!  %v: ", err)
				task.out.respCh <- nil
				task.out.errCh <- err
			} else {
				task.out.respCh <- response
				task.out.errCh <- nil
			}
		}()
	}
}

func init() {
	val := os.Getenv("TEE_FPGA_WORKERS")
	nWorkers, err := strconv.Atoi(val)
	if err != nil {
		logger.Warnf("TEE_FPGA_WORKERS %s is illegal! Now set it to 10.", val)
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
	logger.Infof("grpc.Dial - server adddress is: %s", serverAddr)
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		logger.Fatalf("fail to dial: %v", err)
	}
	//defer conn.Close()
	return pb.NewTeeClient(conn)
}

func Execute(elf []byte, plaintexts [][]byte, feed4decrytions []*pb.Feed4Decryption, nonces [][]byte) (*pb.PlainCiphertexts, error) {
	respCh := make(chan *pb.PlainCiphertexts)
	errCh := make(chan error)
	taskPool <- &task{
		in:&pb.TeeArgs{
			Elf:elf,
			PlainCipherTexts:&pb.PlainCiphertexts{
				Plaintexts:plaintexts,
				Feed4Decryptions:feed4decrytions,
			},
			Nonces:nonces,
		},
		out: &out{
			respCh,
			errCh,
		},
	}
	return <-respCh, <-errCh
}
