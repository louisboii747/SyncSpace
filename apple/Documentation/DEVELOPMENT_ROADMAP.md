# Development roadmap

## Phase 1 — Windows scaffolding

Shared models and protocols, HTTP/WebSocket foundations, explicit mocks, observable state, native starter sources, tests, and handoff docs. This repository state completes the source scaffold, subject to Windows-side Swift validation.

## Phase 2 — First Xcode import

Create workspace/targets, resolve compiler and concurrency diagnostics, configure signing and entitlements, add assets, run previews, package tests, Simulator, physical iPhone, and sandboxed macOS app.

## Phase 3 — Backend connection

Design an authenticated LAN-facing management API without weakening loopback administration. Connect `/device/self`, settings, refresh, live devices, and the three WebSockets. Add compatibility/version negotiation.

## Phase 4 — Pairing

Implement request/verification/accept/reject UI, request expiry, trusted-device removal/blocking, identity-change warnings, and security testing. Reuse backend cryptography; add no fake client crypto.

## Phase 5 — Clipboard and notes

Define real backend contracts, send/receive explicit clipboard text, send notes, persist recent notes, and add opt-in notifications. Keep clipboard surveillance out.

## Phase 6 — Files

Define native upload/download semantics, file importer/panel access, acceptance destinations, progress, cancellation, retry, temporary cleanup, session history, background URLSession evaluation, and security-scoped bookmarks.

## Phase 7 — Platform polish

MenuBarExtra, iOS share extension, drag and drop, notifications, background transfers, optional NWBrowser Bonjour, accessibility audit, icons, localization, security review, signing, notarization, TestFlight, and release preparation.

## Future macOS menu bar

A later `MenuBarExtra` may show connection state, nearby devices, explicit send-clipboard, recent transfers, and Open SyncSpace. It should share the same state container and must not turn clipboard behavior into passive monitoring.
