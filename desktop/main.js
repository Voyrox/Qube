const { app, BrowserWindow, ipcMain, Menu } = require('electron');
const path = require('path');
const Store = require('electron-store');

const store = new Store();

let mainWindow;

// Fixed window size like Docker Desktop
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
    show: false
  });

  // Restore window position if saved
  const windowBounds = store.get('windowBounds');
  if (windowBounds) {
    mainWindow.setBounds(windowBounds);
  }

  // Load the index.html
  mainWindow.loadFile(path.join(__dirname, 'renderer', 'index.html'));

  // Show window when ready
  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
  });

  // Save window position on close
  mainWindow.on('close', () => {
    store.set('windowBounds', mainWindow.getBounds());
  });

  // Open DevTools in development mode
  if (process.argv.includes('--dev')) {
    mainWindow.webContents.openDevTools();
  }

  // Create application menu
  createMenu();
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

// IPC handlers for renderer process
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

// App lifecycle
app.whenReady().then(() => {
  createWindow();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

// Handle errors
process.on('uncaughtException', (error) => {
  console.error('Uncaught exception:', error);
});
