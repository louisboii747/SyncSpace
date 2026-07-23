import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, connectPairingEvents, connectTransferEvents } from '../api'
import type { Device, PairingEvent, PairingRequest, PrivacyPolicy, Settings, Transfer, TransferEvent, TrustedDevice } from '../types'
import { isTerminal } from '../utils'

export interface ToastMessage {
  id: number
  tone: 'success' | 'error' | 'info'
  title: string
  message: string
}

export function useSyncSpace() {
	const [localDevice, setLocalDevice] = useState<Device | null>(null)
  const [devices, setDevices] = useState<Device[]>([])
  const [trusted, setTrusted] = useState<TrustedDevice[]>([])
	const [transfers, setTransfers] = useState<Transfer[]>([])
	const [pairingRequests, setPairingRequests] = useState<PairingRequest[]>([])
	const [settings, setSettings] = useState<Settings | null>(null)
	const [privacyPolicy, setPrivacyPolicy] = useState<PrivacyPolicy | null>(null)
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

  const loadOperationalState = useCallback(async () => {
		const [deviceList, trustList, pairingList, transferList] = await Promise.all([
			api.devices(),
			api.trustedDevices(),
			api.pairingRequests(),
			api.transfers(),
		])
		setDevices(deviceList)
		setTrusted(trustList)
		setPairingRequests(pairingList)
		setTransfers(transferList)
	}, [])

  const load = useCallback(async () => {
    try {
		const [policy, currentSettings, self] = await Promise.all([
			api.privacyPolicy(),
			api.settings(),
			api.self(),
		])
		setPrivacyPolicy(policy)
		setSettings(currentSettings)
		setLocalDevice(self)
		if (policy.accepted) {
			await loadOperationalState()
		} else {
			setDevices([])
			setTrusted([])
			setPairingRequests([])
			setTransfers([])
		}
      setError('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'SyncSpace backend is unavailable')
    } finally {
      setLoading(false)
    }
  }, [loadOperationalState])

  useEffect(() => { void load() }, [load])
  useEffect(() => {
		if (!privacyPolicy?.accepted) return
    const refresh = window.setInterval(() => {
		void Promise.all([api.devices(), api.trustedDevices(), api.pairingRequests()]).then(([nextDevices, nextTrusted, nextPairing]) => {
        setDevices(nextDevices)
			setTrusted(nextTrusted)
			setPairingRequests(nextPairing)
      }).catch(() => undefined)
    }, 10_000)
    return () => window.clearInterval(refresh)
  }, [privacyPolicy?.accepted])

	useEffect(() => {
		if (!privacyPolicy?.accepted) {
			setConnected(false)
			return
		}
		return connectTransferEvents((event: TransferEvent) => {
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
		}, setConnected)
	}, [privacyPolicy?.accepted, settings?.notificationsEnabled, toast])

	useEffect(() => {
		if (!privacyPolicy?.accepted) return
		return connectPairingEvents((event: PairingEvent) => {
			if (event.request) {
				const request = event.request
				setPairingRequests((current) => {
					if (event.type === 'PairingRejected' || request.state === 'rejected' || request.state === 'paired') {
						return current.filter((item) => item.requestId !== request.requestId)
					}
					const index = current.findIndex((item) => item.requestId === request.requestId)
					if (index < 0) return [request, ...current]
					const copy = [...current]
					copy[index] = request
					return copy
				})
				if (event.type === 'PairingRequested' && request.direction === 'incoming' && !request.localConfirmed) {
					toast('info', 'Pairing request received', `Compare the code shown for ${request.deviceName}.`)
				}
			}
			if (event.trustedDevice) {
				setTrusted((current) => [event.trustedDevice!, ...current.filter((item) => item.deviceId !== event.trustedDevice?.deviceId)])
				setPairingRequests((current) => current.filter((item) => item.deviceId !== event.trustedDevice?.deviceId))
				if (event.type === 'PairingAccepted') toast('success', 'Device paired', `${event.trustedDevice.deviceName} is now trusted.`)
			}
		})
	}, [privacyPolicy?.accepted, toast])

	const trustedIDs = useMemo(() => new Set(trusted.filter((item) => !item.blocked && !item.identityKeyChanged).map((item) => item.deviceId)), [trusted])
  const active = useMemo(() => transfers.filter((item) => !isTerminal(item)), [transfers])
  const history = useMemo(() => transfers.filter(isTerminal), [transfers])

  return {
		localDevice, devices, trusted, trustedIDs, pairingRequests, settings, privacyPolicy,
		transfers, active, history, loading, connected, error, toasts, celebration, toast,
		reload: load, setLocalDevice, setTrusted, setPairingRequests, setSettings, setPrivacyPolicy, setTransfers,
  }
}
