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

// sanitizeFilename prevents path traversal attacks
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

// Upload handles streaming uploads with the Commit message pattern:
// 1. metadata -> 2. chunks... -> 3. finish_commit (hash verification)
func (s *Server) Upload(
	ctx context.Context, stream *connect.ClientStream[fileuploadv1.UploadRequest]) (*fileuploadv1.UploadResponse, error) {

	var (
		file      *os.File
		filename  string
		totalSize int64
		hasher    = sha256.New()
	)

	for stream.Receive() {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			if file != nil {
				file.Close()
				os.Remove(filepath.Join(uploadDir, filename))
			}
			return nil, ctx.Err()
		default:
		}

		req := stream.Msg()

		switch payload := req.Payload.(type) {

		case *fileuploadv1.UploadRequest_Metadata:
			if file != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("metadata already received"))
			}

			filename = sanitizeFilename(payload.Metadata.Filename)
			safePath := filepath.Join(uploadDir, filename)
			log.Printf("Upload started: %s (title: %s)", filename, payload.Metadata.Title)

			var err error
			file, err = os.Create(safePath)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			defer file.Close()

		case *fileuploadv1.UploadRequest_Chunk:
			if file == nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("metadata must be sent first"))
			}

			// Write to file AND update hash
			if _, err := file.Write(payload.Chunk); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			hasher.Write(payload.Chunk)
			totalSize += int64(len(payload.Chunk))

		case *fileuploadv1.UploadRequest_FinishCommit:
			if file == nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no file data received"))
			}

			// Final hash verification
			serverHash := hex.EncodeToString(hasher.Sum(nil))
			clientHash := payload.FinishCommit

			log.Printf("Upload complete: %s (%d bytes)", filename, totalSize)
			log.Printf("Hash verification - Server: %s, Client: %s", serverHash, clientHash)

			if serverHash != clientHash {
				log.Printf("HASH MISMATCH! Deleting corrupted file")
				file.Close()
				os.Remove(filepath.Join(uploadDir, filename))
				return nil, connect.NewError(connect.CodeDataLoss, errors.New("checksum mismatch"))
			}

			return &fileuploadv1.UploadResponse{
				Message: "Upload successful and verified",
				Size:    totalSize,
				HashOk:  true,
			}, nil

		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown message type"))
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("stream closed without commit"))
}

// UploadFile handles unary uploads from browser clients
func (s *Server) UploadFile(
	ctx context.Context, req *fileuploadv1.UploadFileRequest) (*fileuploadv1.UploadResponse, error) {

	filename := sanitizeFilename(req.Filename)
	safePath := filepath.Join(uploadDir, filename)

	log.Printf("UploadFile: %s (title: %s)", filename, req.Title)

	// Calculate and verify hash
	hasher := sha256.New()
	hasher.Write(req.Data)
	serverHash := hex.EncodeToString(hasher.Sum(nil))
	hashOk := (serverHash == req.Sha256)

	log.Printf("Hash verification - Server: %s, Client: %s, OK: %v", serverHash, req.Sha256, hashOk)

	// Write file
	if err := os.WriteFile(safePath, req.Data, 0644); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &fileuploadv1.UploadResponse{
		Message: "ok",
		Size:    int64(len(req.Data)),
		HashOk:  hashOk,
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
