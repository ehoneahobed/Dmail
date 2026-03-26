const { app, BrowserWindow, ipcMain } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const fs = require('fs')
const net = require('net')
const http = require('http')

let mainWindow = null
let daemonProcess = null
let daemonPort = 7777
let restartCount = 0
const MAX_RESTARTS = 5
const RESTART_DELAY = 2000

function getDaemonBinaryPath() {
  const platform = process.platform // 'darwin', 'linux', 'win32'
  const arch = process.arch          // 'arm64', 'x64'
  const ext = platform === 'win32' ? '.exe' : ''
  const binaryName = `dmaild${ext}`

  if (app.isPackaged) {
    // Packaged app: binary is in resources/bin/
    return path.join(process.resourcesPath, 'bin', `${platform}-${arch}`, binaryName)
  }

  // Development: look for binary relative to project root
  const devPath = path.join(__dirname, '..', '..', 'dmaild')
  if (fs.existsSync(devPath)) return devPath

  // Also check resources path in dev
  const devResPath = path.join(__dirname, '..', 'resources', 'bin', `${platform}-${arch}`, binaryName)
  if (fs.existsSync(devResPath)) return devResPath

  return devPath // fallback
}

function getDataDir() {
  return path.join(app.getPath('userData'), 'dmail-data')
}

function findFreePort(preferred) {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.once('error', () => {
      // Preferred port in use, find random one
      const s2 = net.createServer()
      s2.listen(0, '127.0.0.1', () => {
        const port = s2.address().port
        s2.close(() => resolve(port))
      })
      s2.once('error', reject)
    })
    server.listen(preferred, '127.0.0.1', () => {
      server.close(() => resolve(preferred))
    })
  })
}

function waitForDaemon(port, timeoutMs = 30000) {
  const start = Date.now()
  return new Promise((resolve, reject) => {
    function check() {
      if (Date.now() - start > timeoutMs) {
        return reject(new Error('Daemon startup timed out'))
      }
      const req = http.get(`http://127.0.0.1:${port}/api/v1/status`, (res) => {
        if (res.statusCode === 200) {
          resolve()
        } else {
          setTimeout(check, 500)
        }
      })
      req.on('error', () => setTimeout(check, 500))
      req.setTimeout(2000, () => {
        req.destroy()
        setTimeout(check, 500)
      })
    }
    check()
  })
}

function startDaemon(port) {
  const binaryPath = getDaemonBinaryPath()
  const dataDir = getDataDir()

  // Ensure data directory exists
  fs.mkdirSync(dataDir, { recursive: true })

  const logFile = path.join(dataDir, 'daemon.log')
  const logStream = fs.createWriteStream(logFile, { flags: 'a' })

  console.log(`Starting daemon: ${binaryPath} --port ${port} --data-dir ${dataDir}`)

  const proc = spawn(binaryPath, ['--port', String(port), '--data-dir', dataDir], {
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env },
  })

  proc.stdout.pipe(logStream)
  proc.stderr.pipe(logStream)

  proc.stdout.on('data', (data) => {
    console.log(`[daemon] ${data.toString().trim()}`)
  })

  proc.stderr.on('data', (data) => {
    console.error(`[daemon] ${data.toString().trim()}`)
  })

  proc.on('exit', (code, signal) => {
    console.log(`Daemon exited: code=${code} signal=${signal}`)
    if (daemonProcess === proc && restartCount < MAX_RESTARTS) {
      restartCount++
      console.log(`Restarting daemon (attempt ${restartCount}/${MAX_RESTARTS})...`)
      setTimeout(() => {
        if (daemonProcess === proc) {
          daemonProcess = null
          startDaemon(port).catch(console.error)
        }
      }, RESTART_DELAY)
    }
  })

  daemonProcess = proc
  return waitForDaemon(port)
}

function stopDaemon() {
  if (daemonProcess) {
    const proc = daemonProcess
    daemonProcess = null
    proc.kill('SIGTERM')
    // Force kill after 5s if still alive
    setTimeout(() => {
      try { proc.kill('SIGKILL') } catch {}
    }, 5000)
  }
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1024,
    height: 720,
    minWidth: 800,
    minHeight: 600,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js'),
    },
    titleBarStyle: 'hiddenInset',
    title: 'Dmail',
  })

  if (process.env.VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(process.env.VITE_DEV_SERVER_URL)
  } else {
    mainWindow.loadFile(path.join(__dirname, '..', 'dist', 'index.html'))
  }

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

app.whenReady().then(async () => {
  try {
    daemonPort = await findFreePort(7777)
    await startDaemon(daemonPort)
    console.log(`Daemon ready on port ${daemonPort}`)
  } catch (err) {
    console.error('Failed to start daemon:', err.message)
    // Continue anyway — frontend will show daemon error screen
  }

  createWindow()
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) createWindow()
})

app.on('before-quit', () => {
  stopDaemon()
})

// IPC handler for renderer to get daemon port
ipcMain.handle('get-daemon-port', () => daemonPort)
