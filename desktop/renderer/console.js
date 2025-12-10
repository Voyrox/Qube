let apiBase = 'http://127.0.0.1:3030';

// Load API base from Electron settings if available
if (window.electron) {
  window.electron.getApiBase().then(base => {
    if (base) apiBase = base;
  });
}

const promptText = "root@Qube:/# ";
let lastCommand = "";
let currentInput = "";
let inputLine = null;
let inputTextSpan = null;
let caretSpan = null;
const terminal = document.getElementById('terminal');
const terminalInput = document.getElementById('terminal-input');

const params = new URLSearchParams(window.location.search);
const containerName = params.get('name');
const listEndpoint = `${apiBase}/list`;

let ws;

function fmtUptime(ts) {
  if (!ts) return '—';
  const now = Math.floor(Date.now() / 1000);
  const d = Math.max(0, now - ts);
  const days = Math.floor(d / 86400);
  const hours = Math.floor((d % 86400) / 3600);
  const minutes = Math.floor((d % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function fmtMem(val) {
  if (val === undefined || val === null) return '—';
  if (val >= 1024) return `${(val / 1024).toFixed(1)}G`;
  return `${val.toFixed(1)}M`;
}

function setStatus(isRunning) {
  const pill = document.getElementById('status-pill');
  if (pill) {
    pill.className = `status ${isRunning ? 'running' : 'stopped'}`;
    pill.innerHTML = `<span class="dot ${isRunning ? 'running' : 'stopped'}"></span> ${isRunning ? 'Running' : 'Stopped'}`;
  }
}

async function loadContainer() {
  if (!containerName) {
    document.getElementById('hero-title').innerText = 'No container selected';
    return;
  }
  try {
    const res = await fetch(listEndpoint);
    const data = await res.json();
    const containers = data.containers || [];
    const info = containers.find(c => c.name === containerName);
    if (!info) {
      document.getElementById('hero-title').innerText = `${containerName} (not found)`;
      return;
    }

    const heroTitle = document.getElementById('hero-title');
    const heroDesc = document.getElementById('hero-desc');
    const imagePill = document.getElementById('image-pill');
    const portsPill = document.getElementById('ports-pill');
    const netPill = document.getElementById('net-pill');
    
    if (heroTitle) heroTitle.innerText = info.name;
    if (heroDesc) heroDesc.innerText = info.command ? info.command.join(' ') : 'Attached console';
    if (imagePill) imagePill.innerText = `image: ${info.image || '—'}`;
    if (portsPill) portsPill.innerText = `ports: ${info.ports || 'none'}`;
    if (netPill) netPill.innerText = `net: ${info.isolated ? 'isolated' : 'shared'}`;

    setStatus(info.pid && info.pid > 0);
    
    const pidEl = document.getElementById('pid-val');
    const uptimeEl = document.getElementById('uptime-val');
    const memEl = document.getElementById('mem-val');
    const cmdEl = document.getElementById('cmd-val');
    
    if (pidEl) pidEl.innerText = info.pid > 0 ? info.pid : '—';
    if (uptimeEl) uptimeEl.innerText = fmtUptime(info.timestamp);
    if (memEl) memEl.innerText = info.memory_mb ? fmtMem(info.memory_mb) : '—';
    if (cmdEl) cmdEl.innerText = info.command && info.command.length ? info.command.join(' ') : '—';

    ensureWebSocket();
  } catch (e) {
    console.error('load error', e);
  }
}

function ensureWebSocket() {
  if (ws || !containerName) return;
  ws = new WebSocket(`${apiBase.replace('http', 'ws')}/eval/${containerName}/command`);
  ws.onopen = () => {
    appendLine('Connected to container console.');
    showPrompt();
  };
  ws.onmessage = (event) => {
    let output = event.data.trim();
    // Filter out command echo (the command we just sent)
    if (lastCommand) {
      const lines = output.split('\n');
      const filtered = lines.filter(line => !line.includes(lastCommand)).join('\n').trim();
      output = filtered;
      lastCommand = "";
    }
    // Show all output including server prompts
    if (output) {
      appendOutput(output);
    }
  };
  ws.onclose = () => appendLine('Connection closed.');
  ws.onerror = (error) => appendLine(`Error: ${error.message || 'WebSocket error'}`);
}

function scrollToBottom() {
  terminal.scrollTop = terminal.scrollHeight;
}

function appendLine(text) {
  const div = document.createElement('div');
  div.className = 'terminal-line';
  div.innerHTML = text;
  terminal.appendChild(div);
  scrollToBottom();
}

function appendOutput(text) {
  const div = document.createElement('div');
  div.className = 'terminal-line';
  div.textContent = text;
  if (inputLine) {
    terminal.insertBefore(div, inputLine);
  } else {
    terminal.appendChild(div);
  }
  scrollToBottom();
}

function focusInput() {
  terminalInput.focus();
}

function showPrompt() {
  // Don't render a custom prompt; just ensure input line is ready
  if (inputLine && inputLine.parentNode) {
    inputLine.parentNode.removeChild(inputLine);
  }
  inputLine = document.createElement('div');
  inputLine.className = 'terminal-line';
  inputTextSpan = document.createElement('span');
  inputTextSpan.textContent = currentInput;
  caretSpan = document.createElement('span');
  caretSpan.className = 'caret';
  inputLine.appendChild(inputTextSpan);
  inputLine.appendChild(caretSpan);
  terminal.appendChild(inputLine);
  scrollToBottom();
  focusInput();
}

terminalInput.addEventListener('input', () => {
  currentInput = terminalInput.value;
  if (inputTextSpan) {
    inputTextSpan.textContent = currentInput;
    scrollToBottom();
  }
});

terminalInput.addEventListener('keydown', (e) => {
  if (!ws) return;
  if (e.key === 'Enter') {
    e.preventDefault();
    const command = currentInput.trim();
    if (command.toLowerCase() === 'clear') {
      terminal.innerHTML = '';
      currentInput = '';
      terminalInput.value = '';
      showPrompt();
      return;
    }
    if (command) {
      lastCommand = command;
      ws.send(command + '\n');
    }
    currentInput = '';
    terminalInput.value = '';
    showPrompt();
  } else if (e.ctrlKey && e.key.toLowerCase() === 'c') {
    e.preventDefault();
    ws.send(String.fromCharCode(3));
  } else if (e.ctrlKey && e.key.toLowerCase() === 'x') {
    e.preventDefault();
    ws.send(String.fromCharCode(24));
  }
});

terminal.addEventListener('click', focusInput);
terminal.addEventListener('focus', focusInput);

document.getElementById('btn-clear').addEventListener('click', () => {
  terminal.innerHTML = '';
  currentInput = '';
  terminalInput.value = '';
  showPrompt();
});
document.getElementById('btn-reload').addEventListener('click', loadContainer);

loadContainer().then(() => {
  focusInput();
});
