import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, connectTransferEvents } from '../api'
import type { Device, PairingRequest, Settings, Transfer, TransferEvent, TrustedDevice } from '../types'

export interface ToastMessage {
  id: number
  tone: 'success' | 'error' | 'info'
  title: string
  message: string
}

export function useSyncSpace() {
  const [devices, setDevices] = useState<Device[]>([])
  const [trusted, setTrusted] = useState<TrustedDevice[]>([])
	const [transfers, setTransfers] = useState<Transfer[]>([])
	const [pairingRequests, setPairingRequests] = useState<PairingRequest[]>([])
	const [settings, setSettings] = useState<Settings | null>(null)
  const [loading, setLoading] = useState(true)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')
  const [toasts, setToasts] = useState<ToastMessage[]>([])
  const [celebration, setCelebration] = useState(0)
  const toastID = useRef(0)

  const toast = useCallback((tone: ToastMessage['tone'], title: string, message: string) => {
    const id = ++toastID.current
    setToasts((current) => [...current.slice(-2), { id, tone, title, message }])
    window.setTimeout(() => setToasts((current) => current.filter((item) => item.id !== id)), 5000)
  }, [])

  const load = useCallback(async () => {
    try {
		const [deviceList, trustList, pairingList, transferList, currentSettings] = await Promise.all([api.devices(), api.trustedDevices(), api.pairingRequests(), api.transfers(), api.settings()])
      setDevices(deviceList)
      setTrusted(trustList)
		setTransfers(transferList)
		setPairingRequests(pairingList)
		setSettings(currentSettings)
      setError('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'SyncSpace backend is unavailable')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])
  useEffect(() => {
    const refresh = window.setInterval(() => {
		void Promise.all([api.devices(), api.trustedDevices(), api.pairingRequests()]).then(([nextDevices, nextTrusted, nextPairing]) => {
        setDevices(nextDevices)
			setTrusted(nextTrusted)
			setPairingRequests(nextPairing)
      }).catch(() => undefined)
    }, 10_000)
    return () => window.clearInterval(refresh)
  }, [])

  useEffect(() => connectTransferEvents((event: TransferEvent) => {
    setTransfers((current) => {
      const index = current.findIndex((item) => item.uuid === event.transfer.uuid)
      if (index < 0) return [event.transfer, ...current]
      const copy = [...current]
      copy[index] = event.transfer
      return copy
    })
    if (event.type === 'Complete') {
      setCelebration((value) => value + 1)
      toast('success', 'Transfer complete', `${event.transfer.filename} arrived safely.`)
		if (settings?.notificationsEnabled && 'Notification' in window && Notification.permission === 'granted') {
        new Notification('SyncSpace transfer complete', { body: event.transfer.filename })
      }
    } else if (event.type === 'Failure') {
      toast('error', 'Transfer needs attention', event.transfer.error || event.transfer.filename)
    }
	}, setConnected), [settings?.notificationsEnabled, toast])

	const trustedIDs = useMemo(() => new Set(trusted.filter((item) => !item.blocked && !item.identityKeyChanged).map((item) => item.deviceId)), [trusted])
  const active = useMemo(() => transfers.filter((item) => !['Completed', 'Cancelled'].includes(item.status)), [transfers])
  const history = useMemo(() => transfers.filter((item) => ['Completed', 'Cancelled'].includes(item.status)), [transfers])

  return {
		devices, trusted, trustedIDs, pairingRequests, settings, transfers, active, history, loading, connected, error,
		toasts, celebration, toast, reload: load, setTrusted, setPairingRequests, setSettings, setTransfers,
  }
}
