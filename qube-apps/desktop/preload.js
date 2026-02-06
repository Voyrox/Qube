const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electron', {
  getSettings: () => ipcRenderer.invoke('get-settings'),
  setSettings: (settings) => ipcRenderer.invoke('set-settings', settings),
  getApiBase: () => ipcRenderer.invoke('get-api-base'),
  setApiBase: (apiBase) => ipcRenderer.invoke('set-api-base', apiBase),
  
  startEvalProcess: (containerName) => ipcRenderer.invoke('start-eval-process', containerName),
  sendEvalCommand: (command) => ipcRenderer.invoke('send-eval-command', command),
  
  onDeepLink: (callback) => ipcRenderer.on('deep-link', (event, data) => callback(data)),
  
  platform: process.platform,
  version: process.versions.electron
});

contextBridge.exposeInMainWorld('qube', {
  version: '1.0.0',
  isDev: process.argv.includes('--dev')
});
