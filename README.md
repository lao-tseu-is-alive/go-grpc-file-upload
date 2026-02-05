# 🚀 Go gRPC File Upload

A production-ready file upload service built with **ConnectRPC** and **Buf**, featuring streaming uploads with SHA-256 integrity verification.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![ConnectRPC](https://img.shields.io/badge/ConnectRPC-Protocol-blue?style=flat)](https://connectrpc.com)
[![Buf](https://img.shields.io/badge/Buf-Managed-purple?style=flat)](https://buf.build)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## ✨ Features

- **🔒 Secure Streaming Uploads** - Path traversal protection, context cancellation handling
- **✅ Integrity Verification** - SHA-256 hash verification using the "Commit" message pattern
- **⚡ Zero-Copy Hashing** - Single-pass file streaming with `io.TeeReader`
- **🌐 Browser Support** - Unary RPC for browsers (Fetch API limitation workaround)
- **📝 Type-Safe Protocol** - Protobuf `oneof` enforces message ordering at compile time

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Upload Protocol                          │
├─────────────────────────────────────────────────────────────────┤
│  Client                                              Server     │
│    │                                                    │       │
│    │── UploadRequest { metadata } ──────────────────────▶│ Create file
│    │                                                    │       │
│    │── UploadRequest { chunk } ─────────────────────────▶│ Write + Hash
│    │── UploadRequest { chunk } ─────────────────────────▶│ Write + Hash
│    │── ...                                              │       │
│    │                                                    │       │
│    │── UploadRequest { finish_commit: sha256 } ─────────▶│ Verify
│    │                                                    │       │
│    │◀─────────────────── UploadResponse ────────────────│ OK / DataLoss
└─────────────────────────────────────────────────────────────────┘
```

### The "Commit" Message Pattern

Traditional streaming faces a paradox: you need the hash *before* sending, but can only calculate it *after* reading. We solve this elegantly:

1. **Metadata Phase** - Client sends filename/title
2. **Streaming Phase** - Client sends chunks; both sides calculate hashes
3. **Commit Phase** - Client sends final hash for verification
4. **Verification** - Server compares hashes, rejects corrupted uploads

## 📁 Project Structure

```
.
├── proto/fileupload/v1/
│   └── fileupload.proto       # Service definition
├── gen/fileupload/v1/         # Generated Go code
├── cmd/
│   ├── server/main.go         # ConnectRPC server
│   ├── client/main.go         # Go streaming client
│   └── http-upload-client/    # Browser client (Vite + TypeScript)
├── buf.yaml                   # Buf module config
└── buf.gen.yaml               # Code generation config
```

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Node.js 20+ (for browser client)
- [Buf CLI](https://buf.build/docs/installation)

### 1. Generate Code

```bash
# Install protoc plugins
npm install -g @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es

# Generate Go and TypeScript code
buf generate
```

### 2. Run the Server

```bash
go run ./cmd/server
# Output: Server on :8080
```

### 3. Upload with Go Client

```bash
go run ./cmd/client myfile.pdf "My Document"

# Output:
# Uploading: myfile.pdf (1048576 bytes)
# Sent metadata
# Sent 1048576 bytes in chunks
# Sending commit with hash: a1b2c3...
# Server response: Upload successful and verified (size: 1048576, hash_ok: true)
```

### 4. Upload from Browser

```bash
cd cmd/http-upload-client
npm install
npm run dev
# Open http://localhost:5173
```

## 📡 Protocol Definition

```protobuf
service FileUploadService {
  // Streaming upload (Go, native clients)
  rpc Upload(stream UploadRequest) returns (UploadResponse);
  
  // Unary upload (browsers)
  rpc UploadFile(UploadFileRequest) returns (UploadResponse);
}

message UploadRequest {
  oneof payload {
    UploadMetadata metadata = 1;  // First: file info
    bytes chunk = 2;              // Middle: file data
    string finish_commit = 3;     // Last: SHA-256 hash
  }
}
```

## 🔐 Security Features

### Path Traversal Protection

```go
// Malicious input: "../../../etc/passwd"
// Sanitized output: "passwd"
func sanitizeFilename(filename string) string {
    base := filepath.Base(filename)
    // Additional sanitization...
}
```

### Context Cancellation

```go
for stream.Receive() {
    select {
    case <-ctx.Done():
        file.Close()
        os.Remove(filepath)  // Cleanup partial upload
        return nil, ctx.Err()
    default:
    }
    // Process message...
}
```

### Hash Verification

Corrupted files are automatically deleted:

```go
if serverHash != clientHash {
    os.Remove(filepath)  // Don't keep corrupted data
    return nil, connect.NewError(connect.CodeDataLoss, "checksum mismatch")
}
```

## ⚡ Performance

The Go client uses `io.TeeReader` for zero-overhead hashing:

```go
hasher := sha256.New()
reader := io.TeeReader(file, hasher)

// Read file once - hash calculated automatically
for {
    n, _ := reader.Read(buf)
    stream.Send(chunk)
}

// Hash ready after streaming
hash := hasher.Sum(nil)
```

**Result:** File is read exactly once, regardless of size.

## 🌐 Browser Limitations

Browsers don't support client-streaming with the Fetch API. The solution:

| Client | RPC Method | Streaming | Hash Verification |
|--------|-----------|-----------|-------------------|
| Go     | `Upload`  | ✅ Yes    | ✅ Commit pattern |
| Browser| `UploadFile` | ❌ No (unary) | ✅ Pre-calculated |

For large browser uploads, consider:
- Chunked unary calls with session management
- Pre-signed URLs (S3/GCS direct upload)

## 🛠️ Development

### Regenerate Protobuf Code

```bash
buf generate
```

### Lint Protobuf

```bash
buf lint
```

### Build All

```bash
go build ./...
cd cmd/http-upload-client && npm run build
```

## 📋 Future Enhancements

- [ ] Configuration via environment variables
- [ ] File size limits
- [ ] Chunked browser uploads for large files
- [ ] Web Worker for browser-side SHA-256
- [ ] Progress reporting via server-sent events

## 📄 License

MIT License - see [LICENSE](LICENSE)

---

Built with ❤️ using [ConnectRPC](https://connectrpc.com) and [Buf](https://buf.build)