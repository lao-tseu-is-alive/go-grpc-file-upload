package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"

	fileuploadv1 "github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1"
	"github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1/fileuploadv1connect"

	"connectrpc.com/connect"
)

type Server struct {
	fileuploadv1connect.UnimplementedFileUploadServiceHandler
}

func (s *Server) Upload(
	ctx context.Context, stream *connect.ClientStream[fileuploadv1.UploadRequest]) (*fileuploadv1.UploadResponse, error) {
	var file *os.File
	var total int64
	var clientHash string
	hasher := sha256.New()

	for stream.Receive() {
		chunk := stream.Msg()

		if file == nil {
			log.Println("Title:", chunk.Title)
			log.Println("Client SHA256:", chunk.Sha256)
			clientHash = chunk.Sha256

			f, err := os.Create("uploads/" + chunk.Filename)
			if err != nil {
				return nil, err
			}
			file = f
			defer file.Close()
		}

		file.Write(chunk.Data)
		hasher.Write(chunk.Data)
		total += int64(len(chunk.Data))
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	serverHash := hex.EncodeToString(hasher.Sum(nil))
	ok := (serverHash == clientHash)

	log.Println("Server SHA256:", serverHash, "OK:", ok)

	resp := &fileuploadv1.UploadResponse{
		Message: "ok",
		Size:    total,
		HashOk:  ok,
	}
	//return connect.NewResponse(resp), nil
	return resp, nil
}

func main() {
	os.MkdirAll("uploads", 0755)

	mux := http.NewServeMux()
	mux.Handle(fileuploadv1connect.NewFileUploadServiceHandler(&Server{}))

	log.Println("Server on :8080")
	http.ListenAndServe(":8080", mux)
}
