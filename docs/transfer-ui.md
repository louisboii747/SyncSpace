# Transfer UI contract

The embedded React interface should feel like a dependable desktop utility. It
helps a person complete one clear task at a time and uses the Go backend as the
source of truth for identity, trust, privacy, transfer progress, and history.

## Language and visual direction

Primary copy uses familiar actions such as **Choose files**, **Pair device**,
**Send files**, **Accept**, and **Try again**. Supporting text should explain what
will happen in a complete sentence. Errors should say what failed and give the
next useful action where one exists.

Avoid promotional one-liners, vague slogans, fake statistics, unexplained
network jargon, all-caps marketing copy, glowing decoration, and repeated cards
inside cards. Device identity, trust, destination, progress, and warnings are
more important than decorative metrics. Motion is restrained and follows the
reduced-motion preference.

Backend terms may still appear in Diagnostics, but normal transfer screens use
plain descriptions. For example, `Negotiating` can be shown as **Agreeing
transfer details**, and `Failed` as **Needs attention** with the actual failure
reason underneath.

## First launch

The interface first loads the policy, settings, and local device identity. If
privacy policy `2026-07-1` has not been accepted, the policy page replaces the
application shell. It explains nearby discovery, direct transfers, local data,
external services, and the choices available to the user.

**Accept and continue** calls the version-checked local API. **Decline** shows a
clear paused state with a way to review the policy again. Hiding the page cannot
enable networking because the backend independently gates discovery, pairing,
WebSockets, and transfers.

The accepted version and timestamp remain visible under Settings > Privacy,
where the full policy can be reopened.

## Navigation and everyday tasks

The main pages have distinct jobs:

- **Home** explains the short path from finding a device to sending files and
  shows useful current activity without inventing speeds or status.
- **Transfers** chooses a paired destination, collects files, opens the send
  review, and lists active or actionable transfers.
- **Devices** separates nearby presence from verified trust. It is the place to
  pair, block, unblock, forget, or begin sending to a device.
- **History** lists terminal completed, cancelled, and failed transfers. Clearing
  the list does not delete files already sent or received.
- **Settings** controls the device name, discoverability, incoming offers,
  receive folder, conflict behaviour, notifications, appearance, motion, and
  privacy review.
- **Diagnostics** exposes service health and support export actions without
  turning the normal workflow into a developer dashboard.

Empty states should be specific. **No nearby devices have been found** is paired
with advice to open SyncSpace on the other computer, use the same local network,
and refresh. A disabled discovery state should point to the exact Settings
toggle rather than looking like a network failure.

## Device identity and trust

Device cards use the editable SyncSpace display name as the primary label and
show the operating-system hostname separately. Platform, availability,
compatibility, last seen, and trust state are supporting details. The stable
device ID and fingerprint are available where a security decision needs them,
not used as friendly names.

Discovery never implies trust. An unpaired device offers **Pair device**, not a
send action. Pairing shows the same six-digit verification code and full
fingerprint on both computers and requires confirmation on both. A compatible,
online, unblocked paired device is marked **Ready** and can be selected as a
destination. The local device is not offered as a destination.

## Sending files

The current browser send flow is deliberately short:

1. Choose an online device marked **Ready**.
2. Choose files, choose a folder, or drop content into SyncSpace.
3. Review the destination display name, hostname/platform, number of selected
   files, total size, relative paths, and each file's size.
4. Read any executable or script warning. This is based on the selected relative
   paths and is a caution, not a malware scan.
5. Add more files, add another folder, remove an individual item, or clear the
   selection while it is still local and unstaged.
6. Choose **Send files** to begin local staging and queue creation, or **Go back**
   without sending anything.

The review appears before a browser staging session is created. Large files are
not loaded into JavaScript merely to render the summary. After confirmation,
file bodies stream through the loopback-only staging API with real upload
progress and then enter the durable backend queue.

Browser selection has honest limits. It provides file bodies and relative paths,
not native absolute source paths. Empty directories cannot be represented in a
browser-staged folder, and modification timestamps and filesystem permissions
are not preserved. The current review also cannot predict name conflicts on the
receiver before that computer checks its destination.

## Receiving files

An incoming offer from a paired device appears as an action card before file
bytes are accepted. It shows the sender display name, hostname, platform,
explicit trusted-device status, transfer summary, item count, total size, a
bounded scrollable per-file manifest preview, destination, conflict choice, and
an executable/script warning when a manifest path uses a known risky extension.
The warning tells the receiver to accept only expected content from someone they
trust; SyncSpace never opens or runs the content.

The receiver can choose a different absolute destination for each offer. The
default is `<home>/Downloads/SyncSpace`. The safe conflict choice is **Keep both
files**, which renames incoming content rather than silently replacing an
existing file. **Accept** begins receipt; **Decline** records a terminal result
without accepting file bytes.

Accepting also asks the backend to check the selected destination volume. If it
reports less free space than the transfer requires, the card stays actionable
and shows the `507` failure in friendly language; no file bytes are accepted.

Sender hostname and platform are optional protocol-v1 presentation fields.
Older stored offers may not contain them, so the card omits those lines instead
of inventing values. The trusted badge is based on backend authorisation against
the paired identity, never on a boolean claimed by the sender.

Turning **Allow new incoming transfer offers** off in Settings returns a clear
rejection to future senders. It does not remove existing history. This is
separate from discoverability, which controls the local mDNS browsing and
advertising session. Neither setting deletes trusted devices or history.

## Progress, recovery, and actions

`GET /api/v1/transfers` and `GET /api/v1/ws/transfers` are the transfer source of
truth. Cards use real backend byte counts for progress, speed, ETA, and current
state. Completion notifications and celebration run only after the backend
publishes `Complete` following whole-file SHA-256 verification; reaching 100%
of bytes alone is not success.

After refresh or a temporary WebSocket disconnect, the frontend reloads current
transfer projections from the API. Errors retain the backend reason and show
retry only where the current direction and state support it. Actions are
state-driven:

- outgoing active transfers can pause and resume;
- failed outgoing transfers can retry from durable chunk state;
- either side can cancel a non-terminal transfer;
- a receiver can accept or decline an awaiting offer.

Coordinated receiver-side pause, resume, and retry are not implemented in the
current embedded UI. They should not be shown as working controls until the
sender/receiver protocol propagates those decisions end to end.

## History and completion actions

History is the terminal subset of the same backend model. Completed, cancelled,
and failed items remain available after a restart until the user clears them.
Clearing history removes eligible metadata records and never deletes a received
file from its chosen destination.

The current headless/embedded-web product does not have native **Open file**,
**Reveal in folder**, **Copy path**, or **Open local data folder** actions. Those
need a trusted platform-shell bridge and must not be simulated with browser-only
claims.

## Accessibility and notifications

Progress bars expose their label, current value, minimum, and maximum to
assistive technology. Modals have labelled dialog semantics, scroll long file
lists inside the available viewport, preserve keyboard focus, and remain usable
at compact desktop widths. Meaning is never conveyed by colour or motion alone.

In-app notices are the primary feedback. Browser/operating-system completion
notifications require browser permission and the local notification setting.
They fire only after verification. Filenames can be sensitive, so notification
permission should be enabled only when lock-screen visibility is acceptable.

Platform integrations remain future native work: Compose share targets and
notifications on Android, SwiftUI/file coordination on Apple platforms, and
desktop shell open/reveal, tray, and installer features on Windows and Linux.
