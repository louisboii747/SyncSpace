import { afterEach, describe, expect, it, vi } from 'vitest'
import { connectTransferEvents } from './api'
import { transferFixture } from './testFixtures'

class FakeSocket {
  static instances: FakeSocket[] = []
  onopen: (() => void) | null = null
  onmessage: ((message: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  constructor(readonly url: string) { FakeSocket.instances.push(this) }
  close() { this.closed = true }
}

describe('transfer WebSocket events', () => {
  afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); FakeSocket.instances = [] })

  it('connects, dispatches valid events, ignores malformed payloads, and closes cleanly', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => undefined)
    vi.stubGlobal('window', { location: { protocol: 'http:', host: '127.0.0.1:8384' }, setTimeout, clearTimeout })
    vi.stubGlobal('WebSocket', FakeSocket)
    const onEvent = vi.fn()
    const onState = vi.fn()
    const stop = connectTransferEvents(onEvent, onState)
    const socket = FakeSocket.instances[0]
    expect(socket.url).toBe('ws://127.0.0.1:8384/ws/transfers')
    socket.onopen?.()
    socket.onmessage?.({ data: JSON.stringify({ type: 'Progress', transfer: transferFixture(), timestamp: '2026-07-06T12:00:00Z' }) })
    socket.onmessage?.({ data: '{not json' })
    expect(onState).toHaveBeenCalledWith(true)
    expect(onEvent).toHaveBeenCalledTimes(1)
    stop()
    expect(socket.closed).toBe(true)
  })
})
