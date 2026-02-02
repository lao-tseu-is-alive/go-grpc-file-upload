package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	fileuploadv1 "github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1"
	"github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1/fileuploadv1connect"

	"net/http"
)

func sha256File(path string) string {
	f, _ := os.Open(path)
	defer f.Close()

	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: client file.txt \"Titre du fichier\"")
	}

	path := os.Args[1]
	title := os.Args[2]
	hash := sha256File(path)

	log.Println("SHA256:", hash)

	f, _ := os.Open(path)
	defer f.Close()

	client := fileuploadv1connect.NewFileUploadServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)

	stream, err := client.Upload(context.Background())
	if err != nil {
		fmt.Printf("error doing client upload %v", err)
	}
	buf := make([]byte, 64*1024)
	first := true

	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := &fileuploadv1.UploadRequest{
				Filename: filepath.Base(path),
				Data:     buf[:n],
			}
			if first {
				chunk.Title = title
				chunk.Sha256 = hash
				first = false
			}
			stream.Send(chunk)
		}
		if err == io.EOF {
			break
		}
	}

	resp, _ := stream.CloseAndReceive()
	log.Printf("Server: %+v\n", resp.Message)
}
