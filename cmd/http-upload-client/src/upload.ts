import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { create } from "@bufbuild/protobuf";
import { FileUploadService, UploadFileRequestSchema } from "./gen/fileupload/v1/fileupload_pb.js";

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
  phase: "hashing" | "uploading" | "complete";
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
  // Phase 1: Calculate hash
  onProgress?.({ phase: "hashing", percent: 0 });

  const hash = await sha256File(file);
  console.log("File SHA-256:", hash);

  onProgress?.({ phase: "hashing", percent: 100 });

  // Phase 2: Read file and upload
  onProgress?.({ phase: "uploading", percent: 0 });

  const buffer = await file.arrayBuffer();
  const data = new Uint8Array(buffer);

  const request = create(UploadFileRequestSchema, {
    filename: file.name,
    title: title,
    sha256: hash,
    data: data,
  });

  try {
    onProgress?.({ phase: "uploading", percent: 50 });

    // Use unary uploadFile RPC (works in browsers, unlike streaming)
    const response = await client.uploadFile(request);

    onProgress?.({ phase: "complete", percent: 100 });

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
    statusDiv.textContent = "Calculating file hash...";
    statusDiv.className = "status";
    progressBar.value = 0;
    progressBar.style.display = "block";

    const result = await uploadFile(file, title, (progress) => {
      if (progress.phase === "hashing") {
        progressBar.value = progress.percent * 0.3; // 0-30%
        statusDiv.textContent = "Calculating SHA-256 hash...";
      } else if (progress.phase === "uploading") {
        progressBar.value = 30 + progress.percent * 0.7; // 30-100%
        statusDiv.textContent = `Uploading ${(file.size / 1024 / 1024).toFixed(1)} MB...`;
      } else {
        progressBar.value = 100;
      }
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
