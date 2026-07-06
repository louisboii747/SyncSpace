import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, connectTransferEvents } from '../api'
import type { Device, Transfer, TransferEvent, TrustedDevice } from '../types'

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
      const [deviceList, trustList, transferList] = await Promise.all([api.devices(), api.trustedDevices(), api.transfers()])
      setDevices(deviceList)
      setTrusted(trustList)
      setTransfers(transferList)
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
      void Promise.all([api.devices(), api.trustedDevices()]).then(([nextDevices, nextTrusted]) => {
        setDevices(nextDevices)
        setTrusted(nextTrusted)
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
      if ('Notification' in window && Notification.permission === 'granted') {
        new Notification('SyncSpace transfer complete', { body: event.transfer.filename })
      }
    } else if (event.type === 'Failure') {
      toast('error', 'Transfer needs attention', event.transfer.error || event.transfer.filename)
    }
  }, setConnected), [toast])

  const trustedIDs = useMemo(() => new Set(trusted.map((item) => item.deviceId)), [trusted])
  const active = useMemo(() => transfers.filter((item) => !['Completed', 'Cancelled'].includes(item.status)), [transfers])
  const history = useMemo(() => transfers.filter((item) => ['Completed', 'Cancelled'].includes(item.status)), [transfers])

  return {
    devices, trusted, trustedIDs, transfers, active, history, loading, connected, error,
    toasts, celebration, toast, reload: load, setTrusted, setTransfers,
  }
}
