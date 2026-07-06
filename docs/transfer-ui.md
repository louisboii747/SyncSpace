# Transfer UI contract

All clients should treat `GET /transfers` plus `GET /ws/transfers` as their
single source of truth. Drag and drop, document pickers, share sheets, and folder
pickers translate selections into `POST /transfers`. Native clients queue real
filesystem paths. The embedded browser client cannot access those paths, so it
streams selections through the loopback staging API before queue promotion.

The primary queue surface should show device, direction, item name/count,
progress, live speed, ETA, and state. Actions are state-driven: pause while
sending/receiving, resume while paused, cancel before a terminal state, and
retry after failure. Incoming `approvalRequired` cards must show sender, item
count, total size, destination picker, and overwrite/rename decision before
calling accept. A `409` with conflict information keeps the approval sheet open.

History is the terminal subset of the same model. Completion notifications and
celebration animation fire only on the `Complete` event, never on 100% progress
before verification. Failure notifications retain the reason and expose retry.
Animations must respect reduced-motion settings, and all progress semantics
must remain available to screen readers.

The embedded React client implements the shared transfer, device, and history
experience. Platform integrations still belong in native projects: Compose
share targets and notifications on Android, SwiftUI/file coordination on Apple
platforms, and desktop shell features on Windows and Linux.
