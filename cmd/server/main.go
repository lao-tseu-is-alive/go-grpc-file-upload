package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/rs/cors"

	fileuploadv1 "github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1"
	"github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1/fileuploadv1connect"
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

// UploadFile handles unary uploads from browser clients (Fetch API doesn't support client streaming)
func (s *Server) UploadFile(
	ctx context.Context, req *fileuploadv1.UploadFileRequest) (*fileuploadv1.UploadResponse, error) {

	log.Println("UploadFile - Title:", req.Title)
	log.Println("UploadFile - Filename:", req.Filename)
	log.Println("UploadFile - Client SHA256:", req.Sha256)

	// Calculate server-side hash
	hasher := sha256.New()
	hasher.Write(req.Data)
	serverHash := hex.EncodeToString(hasher.Sum(nil))
	ok := (serverHash == req.Sha256)

	log.Println("UploadFile - Server SHA256:", serverHash, "OK:", ok)

	// Write file
	file, err := os.Create("uploads/" + req.Filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Write(req.Data)
	if err != nil {
		return nil, err
	}

	return &fileuploadv1.UploadResponse{
		Message: "ok",
		Size:    int64(len(req.Data)),
		HashOk:  ok,
	}, nil
}

func main() {
	os.MkdirAll("uploads", 0755)

	mux := http.NewServeMux()
	mux.Handle(fileuploadv1connect.NewFileUploadServiceHandler(&Server{}))

	// Configure CORS for browser access
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // In production, restrict to specific origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		// Required for Connect streaming
		ExposedHeaders: []string{"Connect-Protocol-Version", "Grpc-Status", "Grpc-Message"},
	})

	log.Println("Server on :8080")
	http.ListenAndServe(":8080", corsHandler.Handler(mux))
}
