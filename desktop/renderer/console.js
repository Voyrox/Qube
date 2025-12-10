let apiBase = 'http://127.0.0.1:3030';

// Load API base from Electron settings if available
if (window.electron) {
  window.electron.getApiBase().then(base => {
    if (base) apiBase = base;
  });
}

const terminal = document.getElementById('terminal');
const terminalInput = document.getElementById('terminal-input');
const backButton = document.getElementById('btn-back');
const dashboardLink = document.getElementById('nav-dashboard');

const params = new URLSearchParams(window.location.search);
const containerName = params.get('name');
const listEndpoint = `${apiBase}/list`;

let currentInput = "";
let inputLine = null;
let inputTextSpan = null;
let caretSpan = null;
let evalProcess = null;

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
  // Start eval process and keep it running
  if (evalProcess || !containerName) return;
  
  appendLine(`Starting eval session for ${containerName}...`);
  
  if (window.electron && window.electron.startEvalProcess) {
    window.electron.startEvalProcess(containerName).then((processId) => {
      evalProcess = processId;
      appendLine('✓ Connected to container. Type commands below:');
      showPrompt();
    }).catch((err) => {
      appendLine(`✗ Error starting eval: ${err}`);
    });
  } else {
    appendLine('Electron API not available');
  }
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
  const lines = text.split('\n');
  for (const line of lines) {
    if (line || line === '') {
      const div = document.createElement('div');
      div.className = 'terminal-line';
      div.textContent = line;
      if (inputLine) {
        terminal.insertBefore(div, inputLine);
      } else {
        terminal.appendChild(div);
      }
    }
  }
  scrollToBottom();
}

function focusInput() {
  terminalInput.focus();
}

function showPrompt() {
  // Remove old input line if exists
  if (inputLine && inputLine.parentNode) {
    inputLine.parentNode.removeChild(inputLine);
  }
  inputLine = document.createElement('div');
  inputLine.className = 'terminal-line';
  
  // Add prompt text
  const promptSpan = document.createElement('span');
  promptSpan.style.color = '#00aa00';
  promptSpan.textContent = '$ ';
  inputLine.appendChild(promptSpan);
  
  inputTextSpan = document.createElement('span');
  inputTextSpan.textContent = currentInput;
  inputLine.appendChild(inputTextSpan);
  
  caretSpan = document.createElement('span');
  caretSpan.className = 'caret';
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
      // Append command to output
      const promptLine = document.createElement('div');
      promptLine.className = 'terminal-line';
      promptLine.textContent = '$ ' + command;
      terminal.appendChild(promptLine);
      
      // Send command to eval process via IPC
      if (window.electron && window.electron.sendEvalCommand) {
        window.electron.sendEvalCommand(command).then((output) => {
          if (output) {
            const lines = output.split('\n');
            for (const line of lines) {
              if (line !== '') {
                const div = document.createElement('div');
                div.className = 'terminal-line';
                div.textContent = line;
                terminal.appendChild(div);
              }
            }
          }
          scrollToBottom();
          showPrompt();
        }).catch((err) => {
          const errLine = document.createElement('div');
          errLine.className = 'terminal-line';
          errLine.textContent = 'Error: ' + err;
          terminal.appendChild(errLine);
          scrollToBottom();
          showPrompt();
        });
      }
    }
    currentInput = '';
    terminalInput.value = '';
  } else if (e.ctrlKey && e.key.toLowerCase() === 'c') {
    e.preventDefault();
    appendLine('^C');
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

// Navigation handlers (CSP-safe; no inline handlers)
if (backButton) {
  backButton.addEventListener('click', () => {
    window.location.href = './index.html';
  });
}
if (dashboardLink) {
  dashboardLink.addEventListener('click', () => {
    window.location.href = './index.html';
  });
}

loadContainer().then(() => {
  focusInput();
});
