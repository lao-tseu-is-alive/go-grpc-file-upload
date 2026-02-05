import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { create } from "@bufbuild/protobuf";
import { FileUploadService, UploadRequestSchema, type UploadRequest } from "./gen/fileupload/v1/fileupload_pb.js";

const transport = createConnectTransport({ baseUrl: "http://localhost:8080" });
const client = createClient(FileUploadService, transport);

/**
 * Calculate SHA-256 hash of a file using Web Crypto API
 */
async function sha256File(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  const hashBuffer = await crypto.subtle.digest("SHA-256", buffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");
}

export interface UploadProgress {
  bytesUploaded: number;
  totalBytes: number;
  percent: number;
}

export interface UploadResult {
  success: boolean;
  message: string;
  size: bigint;
  hashOk: boolean;
}

export async function uploadFile(
  file: File,
  title: string,
  onProgress?: (progress: UploadProgress) => void
): Promise<UploadResult> {
  // Calculate hash first (this reads the entire file)
  onProgress?.({ bytesUploaded: 0, totalBytes: file.size, percent: 0 });

  const hash = await sha256File(file);
  console.log("File SHA-256:", hash);

  const chunkSize = 64 * 1024; // 64KB chunks
  const chunks: UploadRequest[] = [];

  // Read the file and create all chunks upfront
  // (Browser Fetch API doesn't support streaming request bodies)
  const buffer = await file.arrayBuffer();
  const data = new Uint8Array(buffer);

  let offset = 0;
  let first = true;

  while (offset < data.length) {
    const chunk = data.slice(offset, offset + chunkSize);
    offset += chunk.length;

    const request = create(UploadRequestSchema, {
      filename: file.name,
      data: chunk,
    });

    if (first) {
      request.title = title;
      request.sha256 = hash;
      first = false;
    }

    chunks.push(request);

    // Report progress during chunk creation
    onProgress?.({
      bytesUploaded: offset,
      totalBytes: data.length,
      percent: Math.round((offset / data.length) * 50), // First 50% is preparation
    });
  }

  // Create an async generator from the chunks array
  async function* requestStream() {
    let processed = 0;
    for (const chunk of chunks) {
      processed += chunk.data.length;
      onProgress?.({
        bytesUploaded: processed,
        totalBytes: data.length,
        percent: 50 + Math.round((processed / data.length) * 50), // Last 50% is upload
      });
      yield chunk;
    }
  }

  try {
    const response = await client.upload(requestStream());
    return {
      success: true,
      message: response.message,
      size: response.size,
      hashOk: response.hashOk,
    };
  } catch (error) {
    console.error("Upload error:", error);
    return {
      success: false,
      message: error instanceof Error ? error.message : "Unknown error",
      size: BigInt(0),
      hashOk: false,
    };
  }
}

export function setupUpload(element: HTMLButtonElement) {
  const doUpload = async () => {
    const fileInput = document.getElementById("file2Upload") as HTMLInputElement;
    const titleInput = document.getElementById("txtFileTitle") as HTMLInputElement;
    const statusDiv = document.getElementById("uploadStatus") as HTMLDivElement;
    const progressBar = document.getElementById("uploadProgress") as HTMLProgressElement;

    const file = fileInput?.files?.[0];
    if (!file) {
      statusDiv.textContent = "Please select a file first";
      statusDiv.className = "status error";
      return;
    }

    const title = titleInput?.value || file.name;
    element.disabled = true;
    statusDiv.textContent = "Preparing upload...";
    statusDiv.className = "status";
    progressBar.value = 0;
    progressBar.style.display = "block";

    const result = await uploadFile(file, title, (progress) => {
      progressBar.value = progress.percent;
      const phase = progress.percent <= 50 ? "Preparing" : "Uploading";
      statusDiv.textContent = `${phase}: ${progress.percent}% (${(progress.bytesUploaded / 1024 / 1024).toFixed(1)} MB / ${(progress.totalBytes / 1024 / 1024).toFixed(1)} MB)`;
    });

    element.disabled = false;

    if (result.success) {
      statusDiv.textContent = `✓ Upload complete! Size: ${(Number(result.size) / 1024 / 1024).toFixed(2)} MB, Hash verified: ${result.hashOk ? "Yes ✓" : "No ✗"}`;
      statusDiv.className = result.hashOk ? "status success" : "status warning";
    } else {
      statusDiv.textContent = `✗ Upload failed: ${result.message}`;
      statusDiv.className = "status error";
    }
  };

  element.addEventListener("click", () => doUpload());
}
