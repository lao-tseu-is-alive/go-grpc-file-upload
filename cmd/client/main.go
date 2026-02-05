package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	fileuploadv1 "github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1"
	"github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1/fileuploadv1connect"
)

const (
	serverURL = "http://localhost:8080"
	chunkSize = 32 * 1024 // 32KB chunks
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: client <file> <title>")
	}

	path := os.Args[1]
	title := os.Args[2]

	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		log.Fatalf("failed to stat file: %v", err)
	}
	log.Printf("Uploading: %s (%d bytes)", info.Name(), info.Size())

	client := fileuploadv1connect.NewFileUploadServiceClient(
		http.DefaultClient,
		serverURL,
	)

	stream, err := client.Upload(context.Background())
	if err != nil {
		log.Fatalf("failed to create upload stream: %v", err)
	}

	// Phase 1: Send metadata
	err = stream.Send(&fileuploadv1.UploadRequest{
		Payload: &fileuploadv1.UploadRequest_Metadata{
			Metadata: &fileuploadv1.UploadMetadata{
				Filename: filepath.Base(path),
				Title:    title,
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to send metadata: %v", err)
	}
	log.Println("Sent metadata")

	// Phase 2: Stream chunks with TeeReader (calculates hash during read)
	hasher := sha256.New()
	reader := io.TeeReader(f, hasher)
	buf := make([]byte, chunkSize)
	var totalBytes int64

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&fileuploadv1.UploadRequest{
				Payload: &fileuploadv1.UploadRequest_Chunk{
					Chunk: buf[:n],
				},
			}); sendErr != nil {
				log.Fatalf("failed to send chunk: %v", sendErr)
			}
			totalBytes += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}
	}
	log.Printf("Sent %d bytes in chunks", totalBytes)

	// Phase 3: Send finish_commit with calculated hash
	clientHash := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("Sending commit with hash: %s", clientHash)

	err = stream.Send(&fileuploadv1.UploadRequest{
		Payload: &fileuploadv1.UploadRequest_FinishCommit{
			FinishCommit: clientHash,
		},
	})
	if err != nil {
		log.Fatalf("failed to send commit: %v", err)
	}

	// Get response
	resp, err := stream.CloseAndReceive()
	if err != nil {
		log.Fatalf("upload failed: %v", err)
	}

	log.Printf("Server response: %s (size: %d, hash_ok: %v)",
		resp.Message, resp.Size, resp.HashOk)
}
