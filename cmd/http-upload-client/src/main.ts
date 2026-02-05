import './style.css'
import { setupUpload } from './upload.ts'

document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
  <div class="upload-container">
    <h1>📁 File Upload Client</h1>
    <p class="subtitle">Upload files to the gRPC server via ConnectRPC</p>
    
    <fieldset class="upload-form">
      <legend>Select a file to upload</legend>
      
      <div class="form-group">
        <label for="file2Upload">File</label>
        <input type="file" id="file2Upload">
      </div>
      
      <div class="form-group">
        <label for="txtFileTitle">Title</label>
        <input type="text" id="txtFileTitle" placeholder="Enter a title for this file">
      </div>
      
      <progress id="uploadProgress" value="0" max="100" style="display: none;"></progress>
      
      <div id="uploadStatus" class="status"></div>
      
      <button id="cmdRunUpload">Upload to Server</button>
    </fieldset>
  </div>
`

setupUpload(document.querySelector<HTMLButtonElement>('#cmdRunUpload')!)
