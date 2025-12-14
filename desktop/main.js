const { app, BrowserWindow, ipcMain, Menu } = require('electron');
const path = require('path');
const Store = require('electron-store');
const { spawn } = require('child_process');

const store = new Store();

let mainWindow;
let evalProcesses = {};
let deeplinkingUrl;

const WINDOW_WIDTH = 1380;
const WINDOW_HEIGHT = 800;
const MIN_WIDTH = 1024;
const MIN_HEIGHT = 600;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: WINDOW_WIDTH,
    height: WINDOW_HEIGHT,
    minWidth: MIN_WIDTH,
    minHeight: MIN_HEIGHT,
    icon: path.join(__dirname, 'assets', 'icon.png'),
    backgroundColor: '#131b1e',
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      enableRemoteModule: false
    },
    frame: true,
    titleBarStyle: 'default',
    show: false,
    autoHideMenuBar: true
  });

  const windowBounds = store.get('windowBounds');
  if (windowBounds) {
    mainWindow.setBounds(windowBounds);
  }

  mainWindow.loadFile(path.join(__dirname, 'renderer', 'index.html'));

  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
    
    if (deeplinkingUrl) {
      if (typeof deeplinkingUrl === 'string') {
        handleDeepLink(deeplinkingUrl);
      } else {
        mainWindow.webContents.send('deep-link', deeplinkingUrl);
      }
      deeplinkingUrl = null;
    }
  });

  mainWindow.on('close', () => {
    store.set('windowBounds', mainWindow.getBounds());
  });

  if (process.argv.includes('--dev')) {
    mainWindow.webContents.openDevTools();
  }

  Menu.setApplicationMenu(null);
}

function createMenu() {
  const template = [
    {
      label: 'File',
      submenu: [
        {
          label: 'Preferences',
          accelerator: 'CmdOrCtrl+,',
          click: () => {
            // TODO: Open preferences window
          }
        },
        { type: 'separator' },
        {
          label: 'Quit',
          accelerator: 'CmdOrCtrl+Q',
          click: () => {
            app.quit();
          }
        }
      ]
    },
    {
      label: 'View',
      submenu: [
        {
          label: 'Reload',
          accelerator: 'CmdOrCtrl+R',
          click: () => {
            mainWindow.reload();
          }
        },
        {
          label: 'Toggle Developer Tools',
          accelerator: 'CmdOrCtrl+Shift+I',
          click: () => {
            mainWindow.webContents.toggleDevTools();
          }
        },
        { type: 'separator' },
        { role: 'resetZoom' },
        { role: 'zoomIn' },
        { role: 'zoomOut' },
        { type: 'separator' },
        { role: 'togglefullscreen' }
      ]
    },
    {
      label: 'Help',
      submenu: [
        {
          label: 'Documentation',
          click: async () => {
            const { shell } = require('electron');
            await shell.openExternal('https://github.com/Voyrox/Qube');
          }
        },
        {
          label: 'About Qube Desktop',
          click: () => {
            // TODO: Show about dialog
          }
        }
      ]
    }
  ];

  const menu = Menu.buildFromTemplate(template);
  Menu.setApplicationMenu(menu);
}

ipcMain.handle('get-api-base', () => {
  return store.get('apiBase', 'http://127.0.0.1:3030');
});

ipcMain.handle('set-api-base', (event, apiBase) => {
  store.set('apiBase', apiBase);
  return true;
});

ipcMain.handle('get-settings', () => {
  return {
    apiBase: store.get('apiBase', 'http://127.0.0.1:3030'),
    autoRefresh: store.get('autoRefresh', true),
    refreshInterval: store.get('refreshInterval', 1000)
  };
});

ipcMain.handle('set-settings', (event, settings) => {
  Object.entries(settings).forEach(([key, value]) => {
    store.set(key, value);
  });
  return true;
});

ipcMain.handle('start-eval-process', async (event, containerName) => {
  if (evalProcesses[containerName]) {
    return containerName;
  }

  return new Promise((resolve, reject) => {
    const proc = spawn('qube', ['eval', containerName], {
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: true
    });

    let outputBuffer = '';
    
    proc.stdout.on('data', (data) => {
      outputBuffer += data.toString();
    });

    proc.stderr.on('data', (data) => {
      outputBuffer += data.toString();
    });

    proc.on('error', (error) => {
      reject(error.message);
    });

    evalProcesses[containerName] = {
      proc,
      outputBuffer,
      waiters: []
    };

    resolve(containerName);
  });
});

ipcMain.handle('send-eval-command', async (event, command) => {
  const containerName = event.sender.getURL().match(/name=([^&]+)/)?.[1];
  
  if (!containerName || !evalProcesses[containerName]) {
    throw new Error('No active eval process');
  }

  const { proc } = evalProcesses[containerName];
  
  if (!proc || proc.killed) {
    throw new Error('Eval process is dead');
  }
  
  return new Promise((resolve, reject) => {
    let commandOutput = '';
      let timeoutId;
    
    const onData = (data) => {
      commandOutput += data.toString();
    };
    
    proc.stdout.on('data', onData);
    proc.stderr.on('data', onData);
    
    try {
      proc.stdin.write(command + '\n', (err) => {
      if (err) {
          proc.stdout.removeListener('data', onData);
          proc.stderr.removeListener('data', onData);
          clearTimeout(timeoutId);
          reject(new Error('Failed to write command: ' + err.message));
          return;
        }
        
        // Wait a bit for output then resolve
        timeoutId = setTimeout(() => {
          proc.stdout.removeListener('data', onData);
          proc.stderr.removeListener('data', onData);
          resolve(commandOutput.trim());
        }, 150);
      });
    } catch (err) {
      proc.stdout.removeListener('data', onData);
      proc.stderr.removeListener('data', onData);
      clearTimeout(timeoutId);
      reject(new Error('Error sending command: ' + err.message));
    }
  });
});

const PROTOCOL = 'qube';

if (process.defaultApp) {
  if (process.argv.length >= 2) {
    app.setAsDefaultProtocolClient(PROTOCOL, process.execPath, [path.resolve(process.argv[1])]);
  }
} else {
  app.setAsDefaultProtocolClient(PROTOCOL);
}

app.on('second-instance', (event, commandLine, workingDirectory) => {
  if (mainWindow) {
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.focus();
  }

  const url = commandLine.find(arg => arg.startsWith(`${PROTOCOL}://`));
  if (url) {
    handleDeepLink(url);
  }
});

function handleDeepLink(url) {
  if (!url) return;
  
  console.log('Deep link received:', url);
  
  // Parse the URL: qube://container/name or qube://image/id or qube://open?page=containers
  try {
    const urlObj = new URL(url);
    const action = urlObj.hostname;
    const param = urlObj.pathname.replace('/', '');
    const query = Object.fromEntries(urlObj.searchParams);

    if (mainWindow && mainWindow.webContents) {
      mainWindow.webContents.send('deep-link', { action, param, query, fullUrl: url });
    } else {
      deeplinkingUrl = { action, param, query, fullUrl: url };
    }
  } catch (error) {
    console.error('Failed to parse deep link URL:', error);
  }
}

if (process.platform === 'linux' || process.platform === 'win32') {
  const url = process.argv.find(arg => arg.startsWith(`${PROTOCOL}://`));
  if (url) {
    deeplinkingUrl = url;
  }
}

const gotTheLock = app.requestSingleInstanceLock();

if (!gotTheLock) {
  app.quit();
} else {
  app.whenReady().then(() => {
    createWindow();

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) {
        createWindow();
      }
    });
  });
}

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

process.on('uncaughtException', (error) => {
  console.error('Uncaught exception:', error);
});
