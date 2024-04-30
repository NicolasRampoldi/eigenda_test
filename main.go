package main

import (
	"context"
	"crypto/rand"
	"fmt"
	disperserpb "github.com/Layr-Labs/eigenda/api/grpc/disperser"
	"github.com/Layr-Labs/eigenda/clients"
	"github.com/Layr-Labs/eigenda/common"
	"github.com/Layr-Labs/eigenda/common/geth"
	"github.com/Layr-Labs/eigenda/core"
	"github.com/Layr-Labs/eigenda/core/auth"
	"github.com/Layr-Labs/eigenda/core/eth"
	coreindexer "github.com/Layr-Labs/eigenda/core/indexer"
	"github.com/Layr-Labs/eigenda/disperser"
	"github.com/Layr-Labs/eigenda/encoding/kzg"
	"github.com/Layr-Labs/eigenda/encoding/kzg/verifier"
	"github.com/Layr-Labs/eigenda/encoding/utils/codec"
	"github.com/Layr-Labs/eigenda/indexer"
	"github.com/Layr-Labs/eigensdk-go/logging"
	gcommon "github.com/ethereum/go-ethereum/common"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"log"
	"strconv"
	"time"
)

var (
	logger           logging.Logger
	ethClient        common.EthClient
	rpcClient        common.RPCEthClient
	retrievalClient  clients.RetrievalClient
	numConfirmations int = 3
	numRetries           = 0
	cancel           context.CancelFunc
)

func main() {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	privateKeyHex := "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcded"
	signer := auth.NewSigner(privateKeyHex)

	disp := clients.NewDisperserClient(&clients.Config{
		Hostname:          "disperser-holesky.eigenda.xyz",
		Port:              "443",
		Timeout:           10 * time.Second,
		UseSecureGrpcFlag: true,
	}, signer)

	data := make([]byte, 1024)
	_, err := rand.Read(data)

	if err != nil {
		panic(err)
	}

	paddedData := codec.ConvertByPaddingEmptyByte(data)

	blobStatus1, key1, err := disp.DisperseBlob(ctx, paddedData, []uint8{})

	if err != nil {
		panic(err)
	}

	fmt.Println(blobStatus1)
	fmt.Println(key1)

	var reply1 *disperserpb.BlobStatusReply

	for {
		reply1, err = disp.GetBlobStatus(context.Background(), key1)

		if err != nil {
			panic(err)
		}

		blobStatus1, err = disperser.FromBlobStatusProto(reply1.GetStatus())

		if err != nil {
			panic(err)
		}

		if *blobStatus1 != disperser.Confirmed {
			continue
		}

		break
	}

	//err = setupRetrievalClient()
	//
	//retrieved, err := retrievalClient.RetrieveBlob(ctx,
	//	[32]byte(reply1.GetInfo().GetBlobVerificationProof().GetBatchMetadata().GetBatchHeaderHash()),
	//	reply1.GetInfo().GetBlobVerificationProof().GetBlobIndex(),
	//	uint(reply1.GetInfo().GetBlobVerificationProof().GetBatchMetadata().GetBatchHeader().GetReferenceBlockNumber()),
	//	[32]byte(reply1.GetInfo().GetBlobVerificationProof().GetBatchMetadata().GetBatchHeader().GetBatchRoot()),
	//	0, // retrieve blob 1 from quorum 0
	//)

}

func setupRetrievalClient() error {
	ethClientConfig := geth.EthClientConfig{
		RPCURLs:          []string{"http://localhost:8545"},
		PrivateKeyString: "351b8eca372e64f64d514f90f223c5c4f86a04ff3dcead5c27293c547daab4ca", // just random private key
		NumConfirmations: numConfirmations,
		NumRetries:       numRetries,
	}
	client, err := geth.NewMultiHomingClient(ethClientConfig, gcommon.Address{}, logger)
	if err != nil {
		return err
	}
	rpcClient, err := ethrpc.Dial("http://localhost:8545")
	if err != nil {
		log.Fatalln("could not start tcp listener", err)
	}
	tx, err := eth.NewTransactor(logger, client, "0xB4baAfee917fb4449f5ec64804217bccE9f46C67", "0xD4A7E1Bd8015057293f0D0A557088c286942e84b")
	if err != nil {
		return err
	}

	cs := eth.NewChainState(tx, client)
	agn := &core.StdAssignmentCoordinator{}
	nodeClient := clients.NewNodeClient(20 * time.Second)
	srsOrder, err := strconv.Atoi("300000")
	if err != nil {
		return err
	}
	v, err := verifier.NewVerifier(&kzg.KzgConfig{
		G1Path:          "resources/kzg/g1.point.300000",
		G2Path:          "resources/kzg/g2.point.300000",
		G2PowerOf2Path:  "",
		CacheDir:        "resources/kzg/SRSTables",
		NumWorker:       1,
		SRSOrder:        uint64(srsOrder),
		SRSNumberToLoad: uint64(srsOrder),
		Verbose:         true,
		PreloadEncoder:  false,
	}, false)
	if err != nil {
		return err
	}

	indexer, err := coreindexer.CreateNewIndexer(
		&indexer.Config{
			PullInterval: 100 * time.Millisecond,
		},
		client,
		rpcClient,
		"0xD4A7E1Bd8015057293f0D0A557088c286942e84b",
		logger,
	)
	if err != nil {
		return err
	}

	ics, err := coreindexer.NewIndexedChainState(cs, indexer)
	if err != nil {
		return err
	}

	retrievalClient, err = clients.NewRetrievalClient(logger, ics, agn, nodeClient, v, 10)
	if err != nil {
		return err
	}

	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())

	return indexer.Index(ctx)
}
