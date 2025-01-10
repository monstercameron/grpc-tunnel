// public/script.js

// 1. Load the WASM module
const go = new Go();
WebAssembly.instantiateStreaming(fetch("client.wasm"), go.importObject)
  .then((result) => {
    console.log("JS: Successfully loaded and initialized the WASM module.");
    go.run(result.instance);
    // We rely on "onWebSocketOpen" (defined in the WASM) to trigger an initial ListTodos()
  })
  .catch((err) => console.error("JS: Failed to load WASM module:", err));

// 2. UI Elements
const todoInput = document.querySelector("#todoInput");
const addTodoBtn = document.querySelector("#addTodoBtn");
const todosList = document.querySelector("#todosList");

console.log("JS: Initialized UI elements:", {
  todoInput,
  addTodoBtn,
  todosList,
});

// 3. Event Listeners

// Add todo when the button is clicked
addTodoBtn.addEventListener("click", () => {
  const text = todoInput.value.trim();
  if (!text) {
    console.warn("JS: User tried to add an empty todo.");
    showNotification("Please enter a todo.", "warning");
    return;
  }
  console.log(`JS: Adding todo with text: "${text}"`);
  window.CreateTodo(text); // Call WASM function
  todoInput.value = ""; // Clear the input field
});

// Add todo when pressing Enter in the input field
todoInput.addEventListener("keypress", (event) => {
  if (event.key === "Enter") {
    console.log("JS: Enter key pressed, triggering Add Todo action.");
    addTodoBtn.click();
  }
});

// 4. Callback Functions from WASM

// Called when a new todo is created successfully
window.onCreateTodo = (id, text, done) => {
  console.log("JS: Received onCreateTodo callback:", { id, text, done });
  addTodoItem(id, text, done);
  showNotification("Todo added successfully!", "success");
};

// Called for each todo in the list
window.onListTodos = (todos) => {
  console.log("JS: Received todos:", todos);

  // Clear the existing list before adding new todos
  todosList.innerHTML = "";

  // Iterate over each todo and add it to the UI
  todos.forEach((todo) => {
    addTodoItem(todo.id, todo.text, todo.done);
  });
};

// Called when a todo is updated successfully
window.onUpdateTodo = (id, text, done) => {
  console.log("JS: Received onUpdateTodo callback:", { id, text, done });

  // Use document.getElementById instead of querySelector
  const listItem = document.getElementById(id);
  if (!listItem) {
    console.warn("JS: Todo item not found in the UI for update:", id);
    return;
  }
  updateTodoListItem(listItem, id, text, done);
  showNotification("Todo updated successfully!", "success");
};

// Called when a todo is deleted successfully
window.onDeleteTodo = (success) => {
  console.log("JS: Received onDeleteTodo callback. Success:", success);
  if (success) {
    showNotification("Todo deleted successfully!", "success");
    reloadTodos(); // Refresh the list
  } else {
    console.error("JS: Failed to delete the todo.");
    showNotification("Failed to delete the todo.", "error");
  }
};

// 5. WebSocket Connection Confirmation
window.onWebSocketOpen = () => {
  console.log("JS: WebSocket connection established. Fetching todos...");
  // At this point, WSReady is already true. Just call ListTodos.
  window.ListTodos(); 
  showNotification("Connected to the server. Fetching todos...", "info");
};



// 6. Helper Functions

/**
 * Adds or updates a todo item in the UI.
 * @param {string} id - The unique ID of the todo.
 * @param {string} text - The text of the todo.
 * @param {boolean} done - Whether the todo is marked as done.
 */
const addTodoItem = (id, text, done) => {
  console.log("JS: Adding/Updating todo item in the UI:", { id, text, done });

  // Check if the item already exists in the UI
  let existing = document.getElementById(id);
  if (existing) {
    console.log("JS: Todo item already exists, updating it.");
    updateTodoListItem(existing, id, text, done);
    return;
  }

  // Create a new list item for the todo
  const li = document.createElement("li");
  li.id = id; // The ID attribute for getElementById
  li.className = "p-4 bg-indigo-50 rounded-lg shadow flex justify-between items-center";

  // Left side: Checkbox and text
  const leftDiv = document.createElement("div");
  leftDiv.className = "flex items-center space-x-3";

  const checkbox = document.createElement("input");
  checkbox.type = "checkbox";
  checkbox.checked = done;
  checkbox.className = "form-checkbox h-5 w-5 text-indigo-600";
  checkbox.addEventListener("change", () => {
    console.log("JS: Checkbox toggled for todo:", { id, text, checked: checkbox.checked });
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
  editBtn.className =
    "bg-yellow-400 text-white px-3 py-1 rounded hover:bg-yellow-500 transition-colors duration-200";
  editBtn.addEventListener("click", () => {
    const newText = prompt("Edit Todo Text", text);
    if (newText != null && newText.trim() !== "") {
      console.log("JS: Editing todo text:", { id, newText });
      window.UpdateTodo(id, newText.trim(), checkbox.checked);
    }
  });

  const delBtn = document.createElement("button");
  delBtn.textContent = "Delete";
  delBtn.className =
    "bg-red-500 text-white px-3 py-1 rounded hover:bg-red-600 transition-colors duration-200";
  delBtn.addEventListener("click", () => {
    console.log("JS: Deleting todo with ID:", id);
    window.DeleteTodo(id);
  });

  rightDiv.appendChild(editBtn);
  rightDiv.appendChild(delBtn);

  // Assemble the list item
  li.appendChild(leftDiv);
  li.appendChild(rightDiv);
  todosList.appendChild(li);
};

/**
 * Updates an existing todo item in the UI.
 */
const updateTodoListItem = (listItem, id, text, done) => {
  console.log("JS: Updating UI for todo item:", { id, text, done });

  const checkbox = listItem.querySelector("input[type='checkbox']");
  const span = listItem.querySelector("span");

  checkbox.checked = done;
  span.textContent = text;
  span.className = done ? "line-through text-gray-400" : "text-gray-800";
};

/**
 * Reloads the entire todo list.
 */
const reloadTodos = () => {
  console.log("JS: Reloading the todo list.");
  todosList.innerHTML = ""; // Clear the existing list
  window.ListTodos(); // Fetch todos again
};

/**
 * Shows a notification to the user.
 * @param {string} message - The notification message.
 * @param {string} type - The type of notification (success, error, warning, info).
 */
const showNotification = (message, type) => {
  console.log(`JS: Notification - ${message} [${type}]`);

  let container = document.querySelector("#notification-container");
  if (!container) {
    container = document.createElement("div");
    container.id = "notification-container";
    container.className = "fixed top-5 right-5 space-y-2 z-50";
    document.body.appendChild(container);
  }

  const notification = document.createElement("div");
  notification.className = `max-w-sm w-full bg-white shadow-lg rounded-lg pointer-events-auto flex ring-1 ring-black ring-opacity-5 ${getNotificationTypeClasses(
    type
  )} p-4`;

  // Icon
  const icon = document.createElement("div");
  icon.className = "flex-shrink-0";
  const svgNS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(svgNS, "svg");
  svg.setAttribute("class", "h-6 w-6");
  svg.setAttribute("fill", "none");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("stroke", "currentColor");
  const path = document.createElementNS(svgNS, "path");
  path.setAttribute("stroke-linecap", "round");
  path.setAttribute("stroke-linejoin", "round");
  path.setAttribute("stroke-width", "2");

  switch (type) {
    case "success":
      path.setAttribute("d", "M5 13l4 4L19 7");
      svg.classList.add("text-green-600");
      break;
    case "error":
      path.setAttribute("d", "M6 18L18 6M6 6l12 12");
      svg.classList.add("text-red-600");
      break;
    case "warning":
      path.setAttribute("d", "M12 8v4m0 4h.01M21 12c0 4.97-4.03 9-9 9s-9-4.03-9-9 4.03-9 9-9 9 4.03 9 9z");
      svg.classList.add("text-yellow-600");
      break;
    case "info":
    default:
      path.setAttribute("d", "M13 16h-1v-4h-1m1-4h.01M12 2a10 10 0 100 20 10 10 0 000-20z");
      svg.classList.add("text-blue-600");
      break;
  }

  svg.appendChild(path);
  icon.appendChild(svg);

  // Message
  const messageDiv = document.createElement("div");
  messageDiv.className = "ml-3 w-0 flex-1 pt-0.5";
  const messageText = document.createElement("p");
  messageText.className = "text-sm font-medium text-gray-900";
  messageText.textContent = message;
  messageDiv.appendChild(messageText);

  // Close button
  const closeButton = document.createElement("button");
  closeButton.className =
    "ml-4 flex-shrink-0 bg-white rounded-md inline-flex text-gray-400 hover:text-gray-500 focus:outline-none";
  closeButton.setAttribute("aria-label", "Close");
  closeButton.innerHTML = `
    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
      <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
    </svg>
  `;
  closeButton.addEventListener("click", () => {
    container.removeChild(notification);
  });

  // Assemble
  notification.appendChild(icon);
  notification.appendChild(messageDiv);
  notification.appendChild(closeButton);
  container.appendChild(notification);

  // Auto-remove after 5s
  setTimeout(() => {
    if (container.contains(notification)) {
      container.removeChild(notification);
    }
  }, 5000);
};

// Utility to assign border color
const getNotificationTypeClasses = (type) => {
  switch (type) {
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
};
