package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/rs/cors"

	fileuploadv1 "github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1"
	"github.com/lao-tseu-is-alive/go-grpc-file-upload/gen/fileupload/v1/fileuploadv1connect"
)

const uploadDir = "uploads"

// sanitizeFilename prevents path traversal attacks by stripping directory components
func sanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "\\", "_")
	if base == "" || base == "." || base == ".." {
		base = "unnamed_file"
	}
	return base
}

type Server struct {
	fileuploadv1connect.UnimplementedFileUploadServiceHandler
}

// Upload handles streaming uploads using oneof pattern for type-safe protocol
func (s *Server) Upload(
	ctx context.Context, stream *connect.ClientStream[fileuploadv1.UploadRequest]) (*fileuploadv1.UploadResponse, error) {

	var file *os.File
	var total int64
	var clientHash string
	var filename string
	var metadataReceived bool
	hasher := sha256.New()

	for stream.Receive() {
		// Check context for cancellation/timeout
		select {
		case <-ctx.Done():
			if file != nil {
				file.Close()
				os.Remove(filepath.Join(uploadDir, filename))
			}
			return nil, ctx.Err()
		default:
		}

		msg := stream.Msg()

		// Handle oneof content using type switch
		switch content := msg.Content.(type) {
		case *fileuploadv1.UploadRequest_Metadata:
			if metadataReceived {
				return nil, errors.New("metadata already received; protocol violation")
			}
			metadataReceived = true
			meta := content.Metadata

			log.Println("Upload - Title:", meta.Title)
			log.Println("Upload - Client SHA256:", meta.Sha256)
			clientHash = meta.Sha256

			filename = sanitizeFilename(meta.Filename)
			safePath := filepath.Join(uploadDir, filename)
			log.Println("Upload - Saving to:", safePath)

			f, err := os.Create(safePath)
			if err != nil {
				return nil, err
			}
			file = f
			defer file.Close()

		case *fileuploadv1.UploadRequest_Chunk:
			if !metadataReceived {
				return nil, errors.New("chunk received before metadata; protocol violation")
			}
			if file == nil {
				return nil, errors.New("file not initialized")
			}

			chunk := content.Chunk
			if _, err := file.Write(chunk); err != nil {
				return nil, err
			}
			hasher.Write(chunk)
			total += int64(len(chunk))

		default:
			return nil, errors.New("unknown message type")
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	if !metadataReceived {
		return nil, errors.New("no metadata received")
	}

	serverHash := hex.EncodeToString(hasher.Sum(nil))
	ok := (serverHash == clientHash)

	log.Println("Upload - Server SHA256:", serverHash, "OK:", ok)

	return &fileuploadv1.UploadResponse{
		Message: "ok",
		Size:    total,
		HashOk:  ok,
	}, nil
}

// UploadFile handles unary uploads from browser clients
func (s *Server) UploadFile(
	ctx context.Context, req *fileuploadv1.UploadFileRequest) (*fileuploadv1.UploadResponse, error) {

	filename := sanitizeFilename(req.Filename)
	safePath := filepath.Join(uploadDir, filename)

	log.Println("UploadFile - Title:", req.Title)
	log.Println("UploadFile - Safe Path:", safePath)
	log.Println("UploadFile - Client SHA256:", req.Sha256)

	hasher := sha256.New()
	hasher.Write(req.Data)
	serverHash := hex.EncodeToString(hasher.Sum(nil))
	ok := (serverHash == req.Sha256)

	log.Println("UploadFile - Server SHA256:", serverHash, "OK:", ok)

	file, err := os.Create(safePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if _, err = file.Write(req.Data); err != nil {
		return nil, err
	}

	return &fileuploadv1.UploadResponse{
		Message: "ok",
		Size:    int64(len(req.Data)),
		HashOk:  ok,
	}, nil
}

func main() {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(fileuploadv1connect.NewFileUploadServiceHandler(&Server{}))

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		ExposedHeaders:   []string{"Connect-Protocol-Version", "Grpc-Status", "Grpc-Message"},
	})

	log.Println("Server on :8080")
	if err := http.ListenAndServe(":8080", corsHandler.Handler(mux)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
