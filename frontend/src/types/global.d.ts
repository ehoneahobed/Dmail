interface DmailBridge {
  getDaemonPort: () => Promise<number>
}

interface Window {
  dmailBridge?: DmailBridge
  __DMAIL_API_BASE__?: string
}
