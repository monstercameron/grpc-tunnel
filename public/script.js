// script.js

// 1. Load the WASM runtime
const go = new Go();
WebAssembly.instantiateStreaming(fetch("client.wasm"), go.importObject)
  .then(result => {
    console.log("JS: WebAssembly loaded successfully.");
    go.run(result.instance);
  })
  .catch(err => console.error("JS: Failed to load WebAssembly:", err));

// 2. UI event listeners
document.addEventListener("DOMContentLoaded", () => {
  const input = document.getElementById("input");
  const sendBtn = document.getElementById("sendBtn");
  const logDiv = document.getElementById("log");

  const logMessage = (msg) => {
    console.log("JS:", msg);
    const p = document.createElement("p");
    p.textContent = msg;
    logDiv.appendChild(p);
  };

  sendBtn.addEventListener("click", () => {
    const text = input.value.trim();
    if (!text) {
      logMessage("No message entered, ignoring.");
      return;
    }
    logMessage("Sending to WASM: " + text);

    // We call the WASM function that we exposed in main.go
    // This triggers gRPC proto serialization, then WebSocket sending
    if (window.sendMessage) {
      window.sendMessage(text);
    } else {
      logMessage("Error: window.sendMessage is not ready!");
    }

    input.value = "";
  });
});
