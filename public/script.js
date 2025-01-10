// script.js

// 1. Load the WASM module
const go = new Go();
WebAssembly.instantiateStreaming(fetch("client.wasm"), go.importObject)
  .then(result => {
    console.log("JS: WASM loaded successfully.");
    go.run(result.instance);
    // Note: Do NOT call ListTodos() here
    // It will be called after WebSocket connection is open via onWebSocketOpen
  })
  .catch(err => console.error("JS: Failed to load WASM:", err));

// 2. UI Elements
const todoInput = document.getElementById("todoInput");
const addTodoBtn = document.getElementById("addTodoBtn");
const todosList = document.getElementById("todosList");

// 3. Event Listeners
addTodoBtn.addEventListener("click", () => {
  const text = todoInput.value.trim();
  if (!text) {
    showNotification("Please enter a todo.", "warning");
    return;
  }
  console.log("JS: Creating todo:", text);
  window.CreateTodo(text);
  todoInput.value = "";
});

todoInput.addEventListener("keypress", (event) => {
  if (event.key === "Enter") {
    addTodoBtn.click();
  }
});

// 4. Callback Functions from WASM

// Called after creating a todo
window.onCreateTodo = function (id, text, done) {
  console.log("JS: onCreateTodo =>", id, text, done);
  addTodoItem(id, text, done);
  showNotification("Todo added successfully!", "success");
};

// Called for each todo in the list
window.onListTodo = function (id, text, done) {
  console.log("JS: onListTodo =>", id, text, done);
  addTodoItem(id, text, done);
};

// Called after updating a todo
window.onUpdateTodo = function (id, text, done) {
  console.log("JS: onUpdateTodo =>", id, text, done);
  const li = document.getElementById(id);
  if (!li) return;
  updateTodoListItem(li, id, text, done);
  showNotification("Todo updated successfully!", "success");
};

// Called after deleting a todo
window.onDeleteTodo = function (success) {
  console.log("JS: onDeleteTodo => success?", success);
  if (success) {
    showNotification("Todo deleted successfully!", "success");
    // Reload the todos list to reflect deletion
    reloadTodos();
  } else {
    showNotification("Failed to delete the todo.", "error");
  }
};

// 5. WebSocket Connection Confirmation
window.onWebSocketOpen = function() {
  console.log("JS: WebSocket is open, fetching todos...");
  window.ListTodos();
  showNotification("Connected to the server.", "info");
};

// 6. Helper Functions

// Adds a new todo item to the UI
function addTodoItem(id, text, done) {
  // If item already exists, update it
  let existing = document.getElementById(id);
  if (existing) {
    updateTodoListItem(existing, id, text, done);
    return;
  }

  const li = document.createElement("li");
  li.id = id;
  li.className = "p-4 bg-indigo-50 rounded-lg shadow flex justify-between items-center";

  // Left side: Checkbox and text
  const leftDiv = document.createElement("div");
  leftDiv.className = "flex items-center space-x-3";

  const checkbox = document.createElement("input");
  checkbox.type = "checkbox";
  checkbox.checked = done;
  checkbox.className = "form-checkbox h-5 w-5 text-indigo-600";
  checkbox.addEventListener("change", () => {
    // On toggle, update the todo
    window.UpdateTodo(id, text, checkbox.checked);
  });

  const span = document.createElement("span");
  span.textContent = text;
  span.className = done ? "line-through text-gray-400" : "text-gray-800";

  leftDiv.appendChild(checkbox);
  leftDiv.appendChild(span);

  // Right side: Edit and Delete buttons
  const rightDiv = document.createElement("div");
  rightDiv.className = "flex space-x-2";

  const editBtn = document.createElement("button");
  editBtn.textContent = "Edit";
  editBtn.className = "bg-yellow-400 text-white px-3 py-1 rounded hover:bg-yellow-500 transition-colors duration-200";
  editBtn.addEventListener("click", () => {
    const newText = prompt("Edit Todo Text", text);
    if (newText != null && newText.trim() !== "") {
      window.UpdateTodo(id, newText.trim(), checkbox.checked);
    }
  });

  const delBtn = document.createElement("button");
  delBtn.textContent = "Delete";
  delBtn.className = "bg-red-500 text-white px-3 py-1 rounded hover:bg-red-600 transition-colors duration-200";
  delBtn.addEventListener("click", () => {
    console.log("JS: Deleting todo:", id);
    window.DeleteTodo(id);
    // Deletion is handled via the server's response in onDeleteTodo
  });

  rightDiv.appendChild(editBtn);
  rightDiv.appendChild(delBtn);

  // Assemble the list item
  li.appendChild(leftDiv);
  li.appendChild(rightDiv);
  todosList.appendChild(li);
}

// Updates an existing todo item in the UI
function updateTodoListItem(li, id, text, done) {
  // Update the checkbox and text
  const checkbox = li.querySelector("input[type='checkbox']");
  const span = li.querySelector("span");

  checkbox.checked = done;
  span.textContent = text;
  span.className = done ? "line-through text-gray-400" : "text-gray-800";
}

// Reloads the entire todo list
function reloadTodos() {
  todosList.innerHTML = ""; // Clear existing list
  window.ListTodos();       // Fetch and display todos again
}

// Shows a notification to the user
function showNotification(message, type) {
  // Create notification container if it doesn't exist
  let container = document.getElementById("notification-container");
  if (!container) {
    container = document.createElement("div");
    container.id = "notification-container";
    container.className = "fixed top-5 right-5 space-y-2 z-50";
    document.body.appendChild(container);
  }

  // Create notification element
  const notification = document.createElement("div");
  notification.className = `max-w-sm w-full bg-white shadow-lg rounded-lg pointer-events-auto flex ring-1 ring-black ring-opacity-5 ${getNotificationTypeClasses(type)} p-4`;

  // Icon
  const icon = document.createElement("div");
  icon.className = "flex-shrink-0";
  const svgNS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(svgNS, "svg");
  svg.setAttribute("class", "h-6 w-6 text-indigo-600");
  svg.setAttribute("fill", "none");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("stroke", "currentColor");
  const path = document.createElementNS(svgNS, "path");
  path.setAttribute("stroke-linecap", "round");
  path.setAttribute("stroke-linejoin", "round");
  path.setAttribute("stroke-width", "2");

  switch(type) {
    case "success":
      path.setAttribute("d", "M5 13l4 4L19 7");
      svg.classList.replace("text-indigo-600", "text-green-600");
      break;
    case "error":
      path.setAttribute("d", "M6 18L18 6M6 6l12 12");
      svg.classList.replace("text-indigo-600", "text-red-600");
      break;
    case "warning":
      path.setAttribute("d", "M12 8v4m0 4h.01M21 12c0 4.97-4.03 9-9 9s-9-4.03-9-9 4.03-9 9-9 9 4.03 9 9z");
      svg.classList.replace("text-indigo-600", "text-yellow-600");
      break;
    case "info":
    default:
      path.setAttribute("d", "M13 16h-1v-4h-1m1-4h.01M12 2a10 10 0 100 20 10 10 0 000-20z");
      svg.classList.replace("text-indigo-600", "text-blue-600");
      break;
  }

  svg.appendChild(path);
  icon.appendChild(svg);

  // Message
  const messageDiv = document.createElement("div");
  messageDiv.className = "ml-3 w-0 flex-1 pt-0.5";
  const p = document.createElement("p");
  p.className = "text-sm font-medium text-gray-900";
  p.textContent = message;
  messageDiv.appendChild(p);

  // Close button
  const closeBtn = document.createElement("button");
  closeBtn.className = "ml-4 flex-shrink-0 bg-white rounded-md inline-flex text-gray-400 hover:text-gray-500 focus:outline-none";
  closeBtn.setAttribute("aria-label", "Close");
  closeBtn.innerHTML = `
    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
      <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
    </svg>
  `;
  closeBtn.addEventListener("click", () => {
    container.removeChild(notification);
  });

  notification.appendChild(icon);
  notification.appendChild(messageDiv);
  notification.appendChild(closeBtn);
  container.appendChild(notification);

  // Automatically remove the notification after 5 seconds
  setTimeout(() => {
    if (container.contains(notification)) {
      container.removeChild(notification);
    }
  }, 5000);
}

// Determines notification styles based on type
function getNotificationTypeClasses(type) {
  switch(type) {
    case "success":
      return "border-l-4 border-green-500";
    case "error":
      return "border-l-4 border-red-500";
    case "warning":
      return "border-l-4 border-yellow-500";
    case "info":
    default:
      return "border-l-4 border-blue-500";
  }
}
