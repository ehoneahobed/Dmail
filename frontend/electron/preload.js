const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('dmailBridge', {
  getDaemonPort: () => ipcRenderer.invoke('get-daemon-port'),
})
