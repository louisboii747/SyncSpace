import type { SVGProps } from 'react'

export type IconName = 'home' | 'settings' | 'send' | 'devices' | 'history' | 'diagnostics' | 'plus' | 'folder' | 'file' | 'pause' | 'play' | 'close' | 'retry' | 'check' | 'shield' | 'refresh' | 'bell' | 'arrowUp' | 'arrowDown' | 'wifi' | 'more'

const paths: Record<IconName, React.ReactNode> = {
	home: <><path d="m3 11 9-8 9 8"/><path d="M5 10v10h14V10M9 20v-6h6v6"/></>,
	settings: <><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H3v-4h.1a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1a1.7 1.7 0 0 0 1.9.3A1.7 1.7 0 0 0 10 3V3h4v.1a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z"/></>,
  send: <><path d="m4 4 17 8-17 8 3-8-3-8Z"/><path d="M7 12h14"/></>,
  devices: <><rect x="3" y="4" width="13" height="11" rx="2"/><path d="M8 20h3m-1.5-5v5"/><rect x="18" y="8" width="3" height="8" rx="1"/></>,
  history: <><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5m4-1v5l3 2"/></>,
  diagnostics: <><path d="M4 19V9m5 10V5m5 14v-7m5 7V3"/><path d="M2 21h20"/></>,
  plus: <path d="M12 5v14M5 12h14"/>,
  folder: <path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z"/>,
  file: <><path d="M6 2h8l4 4v16H6z"/><path d="M14 2v5h5"/></>,
  pause: <><path d="M8 5v14M16 5v14"/></>,
  play: <path d="m8 5 11 7-11 7V5Z"/>,
  close: <path d="m6 6 12 12M18 6 6 18"/>,
  retry: <><path d="M20 7v5h-5"/><path d="M4 17v-5h5"/><path d="M6.1 8A7 7 0 0 1 18 7l2 5M4 12l2 5a7 7 0 0 0 11.9-1"/></>,
  check: <path d="m5 12 4 4L19 6"/>,
  shield: <><path d="M12 3 4 6v5c0 5 3.3 8.7 8 10 4.7-1.3 8-5 8-10V6l-8-3Z"/><path d="m9 12 2 2 4-4"/></>,
  refresh: <><path d="M20 7v5h-5"/><path d="M4 17v-5h5"/><path d="M6.2 8A7 7 0 0 1 18 7l2 5M4 12l2 5a7 7 0 0 0 11.8-1"/></>,
  bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"/><path d="M10 21h4"/></>,
  arrowUp: <><path d="m7 11 5-5 5 5"/><path d="M12 6v12"/></>,
  arrowDown: <><path d="m7 13 5 5 5-5"/><path d="M12 18V6"/></>,
  wifi: <><path d="M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0"/><circle cx="12" cy="19" r="1"/></>,
  more: <><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/></>,
}

export function Icon({ name, ...props }: { name: IconName } & SVGProps<SVGSVGElement>) {
  return <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...props}>{paths[name]}</svg>
}
