const { contextBridge, ipcRenderer } = require('electron');

// Expose protected methods that allow the renderer process to use
// the ipcRenderer without exposing the entire object
contextBridge.exposeInMainWorld('electron', {
  // Settings API
  getSettings: () => ipcRenderer.invoke('get-settings'),
  setSettings: (settings) => ipcRenderer.invoke('set-settings', settings),
  getApiBase: () => ipcRenderer.invoke('get-api-base'),
  setApiBase: (apiBase) => ipcRenderer.invoke('set-api-base', apiBase),
  
  // Eval process control
  startEvalProcess: (containerName) => ipcRenderer.invoke('start-eval-process', containerName),
  sendEvalCommand: (command) => ipcRenderer.invoke('send-eval-command', command),
  
  // Platform info
  platform: process.platform,
  version: process.versions.electron
});

// Expose a safe API for the renderer
contextBridge.exposeInMainWorld('qube', {
  version: '1.0.0',
  isDev: process.argv.includes('--dev')
});
