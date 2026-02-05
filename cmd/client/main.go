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
	chunkSize = 64 * 1024 // 64KB chunks
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: client <file> <title>")
	}

	path := os.Args[1]
	title := os.Args[2]

	// Open file with proper error handling
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}
	defer f.Close()

	// Get file info for logging
	info, err := f.Stat()
	if err != nil {
		log.Fatalf("failed to stat file: %v", err)
	}
	log.Printf("Uploading: %s (%d bytes)", info.Name(), info.Size())

	// Create hash calculator - will compute hash as we stream
	hasher := sha256.New()
	teeReader := io.TeeReader(f, hasher)

	// Create client
	client := fileuploadv1connect.NewFileUploadServiceClient(
		http.DefaultClient,
		serverURL,
	)

	stream, err := client.Upload(context.Background())
	if err != nil {
		log.Fatalf("failed to create upload stream: %v", err)
	}

	// Send metadata first (using oneof pattern)
	metadataMsg := &fileuploadv1.UploadRequest{
		Content: &fileuploadv1.UploadRequest_Metadata{
			Metadata: &fileuploadv1.FileUploadMetadata{
				Filename: filepath.Base(path),
				Title:    title,
				// SHA256 will be empty - we calculate after streaming
				// Server will verify if we send it, or skip verification if empty
				Sha256: "",
			},
		},
	}
	if err := stream.Send(metadataMsg); err != nil {
		log.Fatalf("failed to send metadata: %v", err)
	}
	log.Println("Sent metadata")

	// Stream file chunks using TeeReader (calculates hash while reading)
	buf := make([]byte, chunkSize)
	var totalBytes int64

	for {
		n, err := teeReader.Read(buf)
		if n > 0 {
			chunkMsg := &fileuploadv1.UploadRequest{
				Content: &fileuploadv1.UploadRequest_Chunk{
					Chunk: buf[:n],
				},
			}
			if sendErr := stream.Send(chunkMsg); sendErr != nil {
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

	// Get final hash (calculated during streaming)
	hash := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("Uploaded %d bytes, SHA256: %s", totalBytes, hash)

	// Close stream and get response
	resp, err := stream.CloseAndReceive()
	if err != nil {
		log.Fatalf("upload failed: %v", err)
	}

	log.Printf("Server response: message=%s, size=%d, hash_ok=%v",
		resp.Message, resp.Size, resp.HashOk)

	// Note: hash_ok will be false because we didn't send the hash upfront
	// To enable server-side verification, we'd need a two-phase approach:
	// 1. Pre-calculate hash (reads file twice), OR
	// 2. Send hash in a final message (requires proto change), OR
	// 3. Accept post-upload verification only
	if !resp.HashOk {
		log.Println("Note: Hash verification skipped (hash calculated after upload)")
	}
}
