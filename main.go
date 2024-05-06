package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/Layr-Labs/eigenda/encoding/utils/codec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"time"

	"github.com/Layr-Labs/eigenda/api/grpc/disperser"
)

func main() {
	ctx := context.Background()

	config := &tls.Config{}
	credential := credentials.NewTLS(config)

	clientConn, err := grpc.NewClient("disperser-holesky.eigenda.xyz:443", grpc.WithTransportCredentials(credential))
	if err != nil {
		fmt.Println(err)
		return
	}
	disperserClient := disperser.NewDisperserClient(clientConn)

	data := codec.ConvertByPaddingEmptyByte([]byte("Hello, World!"))
	disperseBlobReq := &disperser.DisperseBlobRequest{
		Data:      data,
		AccountId: "f39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
	}

	disperseBlob, err := disperserClient.DisperseBlob(ctx, disperseBlobReq)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("disperseBlob", disperseBlob)

	getBlobStatusReq := &disperser.BlobStatusRequest{
		RequestId: disperseBlob.RequestId,
	}

	status, err := disperserClient.GetBlobStatus(ctx, getBlobStatusReq)
	if err != nil {
		return
	}

	for status.Status == disperser.BlobStatus_PROCESSING {
		time.Sleep(10 * time.Second)
		fmt.Println("Processing...")
		status, err = disperserClient.GetBlobStatus(ctx, getBlobStatusReq)
		if err != nil {
			fmt.Println("error getting blob status", err)
			return
		}
	}

	blobVerificationProof := status.GetInfo().GetBlobVerificationProof()

	req := disperser.RetrieveBlobRequest{
		BatchHeaderHash: blobVerificationProof.GetBatchMetadata().GetBatchHeaderHash(),
		BlobIndex:       blobVerificationProof.GetBlobIndex(),
	}

	blob, err := disperserClient.RetrieveBlob(ctx, &req)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(blob)
}
