import { useEffect, useMemo, useRef, useState } from "react";
import { api, stageAndQueue } from "./api";
import { DiagnosticsPage } from "./components/DiagnosticsPage";
import { Icon } from "./components/Icon";
import type { IconName } from "./components/Icon";
import { TransferCard } from "./components/TransferCard";
import { useSyncSpace } from "./hooks/useSyncSpace";
import type {
  ConflictPolicy,
  Device,
  LocalFile,
  PairingRequest,
  PrivacyPolicy,
  PrivacyPolicySection,
  Settings,
  Transfer,
  TrustedDevice,
  UploadProgress,
  View,
} from "./types";
import {
  cleanRelativePath,
  formatBytes,
  isActive,
  isPotentiallyExecutable,
  isTerminal,
  topLevelRoots,
} from "./utils";

const CURRENT_PROTOCOL_VERSION = 1;

const emptyUpload: UploadProgress = {
  active: false,
  completedBytes: 0,
  totalBytes: 0,
  currentName: "",
  completedFiles: 0,
  totalFiles: 0,
};

const pageCopy: Record<
  View,
  { label: string; title: string; description: string }
> = {
  home: {
    label: "Overview",
    title: "Send files to devices nearby",
    description:
      "Choose a device you trust, then send files directly over your local network.",
  },
  transfers: {
    label: "Transfers",
    title: "Send and receive files",
    description:
      "Choose where the files should go, review them, and follow their progress here.",
  },
  devices: {
    label: "Devices",
    title: "Nearby and paired devices",
    description:
      "Pair new devices, check which ones are available, or remove access to those you no longer need.",
  },
  history: {
    label: "History",
    title: "Previous transfers",
    description:
      "Review completed, cancelled, and failed transfers stored on this device.",
  },
  settings: {
    label: "Settings",
    title: "Choose how SyncSpace works",
    description:
      "Manage this device, incoming files, notifications, appearance, and privacy.",
  },
  diagnostics: {
    label: "Diagnostics",
    title: "Check SyncSpace services",
    description:
      "Review local service health and export information that can help with troubleshooting.",
  },
};

interface PendingSend {
  deviceId: string;
  files: LocalFile[];
}

export default function App() {
  const sync = useSyncSpace();
  const [view, setView] = useState<View>("home");
  const [selectedDevice, setSelectedDevice] = useState("");
  const [conflictPolicy, setConflictPolicy] =
    useState<ConflictPolicy>("rename");
  const [dragging, setDragging] = useState(false);
  const [upload, setUpload] = useState<UploadProgress>(emptyUpload);
  const [pairTarget, setPairTarget] = useState<Device | null>(null);
  const [pairing, setPairing] = useState(false);
  const [pairingRequest, setPairingRequest] = useState<PairingRequest | null>(
    null,
  );
  const [pendingSend, setPendingSend] = useState<PendingSend | null>(null);
  const [showPrivacyPolicy, setShowPrivacyPolicy] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const folderInput = useRef<HTMLInputElement>(null);
  const dragDepth = useRef(0);
  const appendToReview = useRef(false);

  const capableDevices = useMemo(
    () =>
      sync.devices.filter(
        (device) =>
          device.online &&
          device.transferCapability &&
          device.supportedProtocolVersion === CURRENT_PROTOCOL_VERSION,
      ),
    [sync.devices],
  );
  const readyDevices = useMemo(
    () =>
      capableDevices.filter((device) => sync.trustedIDs.has(device.deviceId)),
    [capableDevices, sync.trustedIDs],
  );
  const selected = readyDevices.find(
    (device) => device.deviceId === selectedDevice,
  );

  useEffect(() => {
    if (selected) return;
    setSelectedDevice(readyDevices[0]?.deviceId || "");
  }, [readyDevices, selected]);

  useEffect(() => {
    const incoming = sync.pairingRequests.find(
      (request) =>
        request.direction === "incoming" &&
        ["pending", "confirming"].includes(request.state),
    );
    if (incoming && !pairingRequest) setPairingRequest(incoming);
  }, [pairingRequest, sync.pairingRequests]);

  useEffect(() => {
    if (!sync.settings) return;
    document.documentElement.dataset.theme = sync.settings.appearance;
    document.documentElement.classList.toggle(
      "reduced-motion",
      sync.settings.reducedMotion,
    );
    setConflictPolicy(sync.settings.conflictPolicy);
  }, [sync.settings]);

  const onTransferChanged = (changed: Transfer) => {
    sync.setTransfers((current) => {
      const index = current.findIndex((item) => item.uuid === changed.uuid);
      if (index < 0) return [changed, ...current];
      const copy = [...current];
      copy[index] = changed;
      return copy;
    });
  };

  const prepareFiles = (files: LocalFile[]) => {
    if (!selected) {
      sync.toast(
        "info",
        "Choose a paired device first",
        "Open Transfers, select a device marked Ready, and then choose your files.",
      );
      return;
    }
    if (!files.length) {
      sync.toast(
        "info",
        "No files were selected",
        "Choose at least one readable file or folder to continue.",
      );
      return;
    }
    setPendingSend({ deviceId: selected.deviceId, files });
  };

  const receiveSelectedFiles = (files: LocalFile[]) => {
    if (appendToReview.current && pendingSend) {
      setPendingSend({
        ...pendingSend,
        files: mergeLocalFiles(pendingSend.files, files),
      });
    } else {
      prepareFiles(files);
    }
    appendToReview.current = false;
  };

  const confirmSend = async () => {
    if (!pendingSend) return;
    if (!pendingSend.files.length) {
      sync.toast(
        "info",
        "Choose at least one file",
        "Add files or a folder to the review before starting the transfer.",
      );
      return;
    }
    const destination = readyDevices.find(
      (device) => device.deviceId === pendingSend.deviceId,
    );
    if (!destination) {
      sync.toast(
        "error",
        "The destination is no longer available",
        "Reconnect or pair the device, then review the transfer again.",
      );
      return;
    }
    try {
      const queued = await stageAndQueue(
        pendingSend.files,
        destination.deviceId,
        conflictPolicy,
        setUpload,
      );
      onTransferChanged(queued);
      setPendingSend(null);
      sync.toast(
        "success",
        "Transfer added to the queue",
        `${pendingSend.files.length} ${pendingSend.files.length === 1 ? "file is" : "files are"} ready for ${destination.deviceName}.`,
      );
    } catch (error) {
      sync.toast(
        "error",
        "The transfer could not be prepared",
        error instanceof Error
          ? error.message
          : "SyncSpace could not stage the selected files.",
      );
    }
  };

  const startPairing = async () => {
    if (!pairTarget) return;
    setPairing(true);
    try {
      const request = await api.requestPairing(pairTarget.deviceId);
      sync.setPairingRequests((current) => [
        ...current.filter((item) => item.requestId !== request.requestId),
        request,
      ]);
      setPairingRequest(request);
      setPairTarget(null);
    } catch (error) {
      sync.toast(
        "error",
        "Pairing could not start",
        error instanceof Error
          ? error.message
          : "SyncSpace could not contact the other device.",
      );
    } finally {
      setPairing(false);
    }
  };

  const refreshDevices = async () => {
    try {
      await api.refreshDevices();
      await sync.reload();
      sync.toast(
        "success",
        "Device list refreshed",
        "SyncSpace checked the local network again.",
      );
    } catch (error) {
      sync.toast(
        "error",
        "Devices could not be refreshed",
        error instanceof Error
          ? error.message
          : "Check the network connection and try again.",
      );
    }
  };

  const enterDrag = (event: React.DragEvent) => {
    event.preventDefault();
    dragDepth.current += 1;
    if (event.dataTransfer.types.includes("Files")) setDragging(true);
  };

  const leaveDrag = (event: React.DragEvent) => {
    event.preventDefault();
    dragDepth.current -= 1;
    if (dragDepth.current <= 0) {
      dragDepth.current = 0;
      setDragging(false);
    }
  };

  const drop = async (event: React.DragEvent) => {
    event.preventDefault();
    dragDepth.current = 0;
    setDragging(false);
    try {
      const files = await filesFromDrop(event.dataTransfer);
      if (pendingSend) {
        setPendingSend({
          ...pendingSend,
          files: mergeLocalFiles(pendingSend.files, files),
        });
      } else {
        prepareFiles(files);
      }
    } catch (error) {
      sync.toast(
        "error",
        "Some files could not be read",
        error instanceof Error
          ? error.message
          : "Choose the files again and check their permissions.",
      );
    }
  };

  if (sync.loading && !sync.privacyPolicy) {
    return <StartupState />;
  }

  if (!sync.privacyPolicy) {
    return (
      <StartupState
        error={
          sync.error ||
          "The local SyncSpace service did not return its privacy policy."
        }
        onRetry={() => void sync.reload()}
      />
    );
  }

  if (!sync.privacyPolicy.accepted) {
    return (
      <PrivacyGate
        policy={sync.privacyPolicy}
        onAccept={async () => {
          const accepted = await api.acceptPrivacyPolicy(
            sync.privacyPolicy!.version,
          );
          sync.setPrivacyPolicy(accepted);
          await sync.reload();
        }}
      />
    );
  }

  const activeCount = sync.active.filter(isActive).length;
  const totalSpeed = sync.active.reduce((sum, item) => sum + item.speed, 0);
  const currentPage = pageCopy[view];
  const localInitials = initials(
    sync.localDevice?.deviceName || sync.settings?.deviceName || "SyncSpace",
  );

  return (
    <div
      className="app-shell"
      onDragEnter={enterDrag}
      onDragLeave={leaveDrag}
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => void drop(event)}
    >
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            <i />
            <i />
            <i />
          </span>
          <span>SyncSpace</span>
        </div>
        <nav aria-label="Main navigation">
          <NavButton
            icon="home"
            label="Home"
            active={view === "home"}
            onClick={() => setView("home")}
          />
          <NavButton
            icon="send"
            label="Transfers"
            active={view === "transfers"}
            badge={sync.active.length || undefined}
            onClick={() => setView("transfers")}
          />
          <NavButton
            icon="devices"
            label="Devices"
            active={view === "devices"}
            badge={sync.pairingRequests.length || undefined}
            onClick={() => setView("devices")}
          />
          <NavButton
            icon="history"
            label="History"
            active={view === "history"}
            onClick={() => setView("history")}
          />
          <NavButton
            icon="settings"
            label="Settings"
            active={view === "settings"}
            onClick={() => setView("settings")}
          />
          <NavButton
            icon="diagnostics"
            label="Diagnostics"
            active={view === "diagnostics"}
            onClick={() => setView("diagnostics")}
          />
        </nav>

        <div className="sidebar-status">
          <div className="radar" aria-hidden="true">
            <span />
            <span />
            <i />
          </div>
          <strong>
            {!sync.settings?.discoverable
              ? "Local-network discovery is off"
              : readyDevices.length
                ? `${readyDevices.length} ready to receive`
                : "No paired device is ready"}
          </strong>
          <span>
            {sync.settings?.discoverable
              ? `${capableDevices.length} compatible ${capableDevices.length === 1 ? "device" : "devices"} nearby`
              : "Enable discovery in Settings to find devices"}
          </span>
        </div>
        <div className="connection-row">
          <span className={sync.connected ? "online" : ""} />
          {sync.connected
            ? "Transfer updates are connected"
            : "Reconnecting to transfer updates"}
        </div>
      </aside>

      <main>
        <header className="topbar">
          <div className="page-intro">
            <span className="eyebrow">{currentPage.label}</span>
            <h1>{currentPage.title}</h1>
            <p className="page-description">{currentPage.description}</p>
          </div>
          <div className="top-actions">
            <div
              className={`topbar-connection ${sync.connected ? "online" : ""}`}
              role="status"
            >
              <i />
              <span>{sync.connected ? "Connected" : "Reconnecting"}</span>
            </div>
            <button
              className="icon-button"
              aria-label="Review notification settings"
              title="Review notification settings"
              onClick={() => setView("settings")}
            >
              <Icon name="bell" />
            </button>
            <div
              className="profile"
              aria-label={`This device: ${sync.localDevice?.deviceName || "SyncSpace"}`}
            >
              {localInitials}
            </div>
          </div>
        </header>

        {sync.error && (
          <div className="backend-error" role="alert">
            <div>
              <strong>The local SyncSpace service is not responding</strong>
              <span>{sync.error}</span>
            </div>
            <button className="button ghost" onClick={() => void sync.reload()}>
              Try again
            </button>
          </div>
        )}

        {sync.loading ? (
          <div className="app-loading glass-panel" aria-busy="true">
            <span className="loading-pulse" />
            <strong>Loading information from this SyncSpace device…</strong>
          </div>
        ) : view === "home" ? (
          <HomeView
            devices={capableDevices}
            trusted={sync.trusted}
            transfers={sync.transfers}
            onSend={() => setView("transfers")}
            onDevices={() => setView("devices")}
          />
        ) : null}

        {!sync.loading && view === "transfers" && (
          <TransferView
            devices={capableDevices}
            discoveryEnabled={sync.settings?.discoverable ?? false}
            trustedIDs={sync.trustedIDs}
            selectedDevice={selectedDevice}
            onSelect={setSelectedDevice}
            active={sync.active}
            activeCount={activeCount}
            totalSpeed={totalSpeed}
            onBrowseFiles={() => {
              appendToReview.current = false;
              fileInput.current?.click();
            }}
            onBrowseFolder={() => {
              appendToReview.current = false;
              folderInput.current?.click();
            }}
            onPair={setPairTarget}
            onRefresh={() => void refreshDevices()}
            onOpenSettings={() => setView("settings")}
            onChanged={onTransferChanged}
            onError={(message) =>
              sync.toast("error", "The transfer action failed", message)
            }
            defaultDestination={sync.settings?.defaultDownloadDirectory || ""}
            defaultConflictPolicy={sync.settings?.conflictPolicy || "rename"}
          />
        )}

        {!sync.loading && view === "devices" && (
          <DevicesView
            devices={sync.devices}
            discoveryEnabled={sync.settings?.discoverable ?? false}
            localDevice={sync.localDevice}
            trusted={sync.trusted}
            onSelect={(device) => {
              setSelectedDevice(device.deviceId);
              setView("transfers");
            }}
            onPair={setPairTarget}
            onRefresh={() => void refreshDevices()}
            onOpenSettings={() => setView("settings")}
            onTrustedChanged={(device) =>
              sync.setTrusted((current) => [
                ...current.filter((item) => item.deviceId !== device.deviceId),
                device,
              ])
            }
            onForgot={(id) =>
              sync.setTrusted((current) =>
                current.filter((item) => item.deviceId !== id),
              )
            }
            onError={(message) =>
              sync.toast("error", "The device action failed", message)
            }
          />
        )}

        {!sync.loading && view === "history" && (
          <HistoryView
            transfers={sync.history}
            onChanged={onTransferChanged}
            onError={(message) =>
              sync.toast("error", "The history action failed", message)
            }
            onCleared={() =>
              sync.setTransfers((current) =>
                current.filter((item) => !isTerminal(item)),
              )
            }
            defaultDestination={sync.settings?.defaultDownloadDirectory || ""}
            defaultConflictPolicy={sync.settings?.conflictPolicy || "rename"}
          />
        )}

        {!sync.loading && view === "settings" && sync.settings && (
          <SettingsView
            settings={sync.settings}
            localDevice={sync.localDevice}
            policy={sync.privacyPolicy}
            onViewPrivacy={() => setShowPrivacyPolicy(true)}
            onChanged={sync.setSettings}
            onLocalDeviceChanged={sync.setLocalDevice}
            onToast={sync.toast}
          />
        )}

        {!sync.loading && view === "diagnostics" && (
          <DiagnosticsPage websocketConnected={sync.connected} />
        )}
      </main>

      <input
        ref={fileInput}
        className="visually-hidden"
        type="file"
        multiple
        onChange={(event) => {
          receiveSelectedFiles(filesFromList(event.currentTarget.files));
          event.currentTarget.value = "";
        }}
      />
      <input
        ref={folderInput}
        className="visually-hidden"
        type="file"
        multiple
        {...{ webkitdirectory: "", directory: "" }}
        onChange={(event) => {
          receiveSelectedFiles(filesFromList(event.currentTarget.files));
          event.currentTarget.value = "";
        }}
      />

      {dragging && (
        <div className="drop-overlay" role="status">
          <div className="drop-orb">
            <Icon name="plus" />
          </div>
          <h2>
            {selected
              ? "Drop the files to review them"
              : "Choose a paired device first"}
          </h2>
          <p>
            {selected
              ? `You will check the file list before anything is sent to ${selected.deviceName}.`
              : "Open Transfers and choose a device marked Ready before dropping files."}
          </p>
        </div>
      )}

      {pairTarget && (
        <PairingIntroduction
          device={pairTarget}
          busy={pairing}
          onClose={() => setPairTarget(null)}
          onContinue={() => void startPairing()}
        />
      )}

      {pairingRequest && (
        <PairingModal
          request={pairingRequest}
          onClose={() => setPairingRequest(null)}
          onUpdated={(request) => {
            setPairingRequest(request);
            sync.setPairingRequests((current) => [
              ...current.filter((item) => item.requestId !== request.requestId),
              request,
            ]);
          }}
          onPaired={(trusted) => {
            sync.setTrusted((current) => [
              ...current.filter((item) => item.deviceId !== trusted.deviceId),
              trusted,
            ]);
            setSelectedDevice(trusted.deviceId);
            sync.setPairingRequests((current) =>
              current.filter(
                (item) => item.requestId !== pairingRequest.requestId,
              ),
            );
            setPairingRequest(null);
            sync.toast(
              "success",
              "Device paired",
              `${trusted.deviceName} can now exchange files with this device.`,
            );
          }}
          onError={(message) =>
            sync.toast("error", "Pairing could not be completed", message)
          }
        />
      )}

      {pendingSend && (
        <SendReviewModal
          files={pendingSend.files}
          device={readyDevices.find(
            (device) => device.deviceId === pendingSend.deviceId,
          )}
          busy={upload.active}
          onAddFiles={() => {
            appendToReview.current = true;
            fileInput.current?.click();
          }}
          onAddFolder={() => {
            appendToReview.current = true;
            folderInput.current?.click();
          }}
          onRemove={(index) =>
            setPendingSend((current) =>
              current
                ? {
                    ...current,
                    files: current.files.filter(
                      (_file, fileIndex) => fileIndex !== index,
                    ),
                  }
                : null,
            )
          }
          onClear={() =>
            setPendingSend((current) =>
              current ? { ...current, files: [] } : null,
            )
          }
          onCancel={() => setPendingSend(null)}
          onConfirm={() => void confirmSend()}
        />
      )}

      {showPrivacyPolicy && (
        <PrivacyPolicyDialog
          policy={sync.privacyPolicy}
          onClose={() => setShowPrivacyPolicy(false)}
        />
      )}

      {upload.active && <UploadNotice upload={upload} />}

      <div className="toast-stack" aria-live="polite">
        {sync.toasts.map((toast) => (
          <div className={`toast ${toast.tone}`} key={toast.id}>
            <span>
              <Icon
                name={
                  toast.tone === "success"
                    ? "check"
                    : toast.tone === "error"
                      ? "close"
                      : "wifi"
                }
              />
            </span>
            <div>
              <strong>{toast.title}</strong>
              <p>{toast.message}</p>
            </div>
          </div>
        ))}
      </div>

      {sync.celebration > 0 && !sync.settings?.reducedMotion && (
        <Celebration key={sync.celebration} />
      )}
    </div>
  );
}

function StartupState({
  error,
  onRetry,
}: {
  error?: string;
  onRetry?: () => void;
}) {
  return (
    <main className="startup-screen">
      <section
        className="startup-card glass-panel"
        aria-busy={error ? undefined : "true"}
      >
        <div className="brand startup-brand">
          <span className="brand-mark" aria-hidden="true">
            <i />
            <i />
            <i />
          </span>
          <span>SyncSpace</span>
        </div>
        {error ? (
          <>
            <h1>SyncSpace could not start yet</h1>
            <p>{error}</p>
            {onRetry && (
              <button className="button primary" onClick={onRetry}>
                Try again
              </button>
            )}
          </>
        ) : (
          <>
            <span className="loading-pulse" />
            <h1>Starting SyncSpace on this device</h1>
            <p>
              The local service is loading your settings and privacy
              information.
            </p>
          </>
        )}
      </section>
    </main>
  );
}

export function PrivacyGate({
  policy,
  onAccept,
}: {
  policy: PrivacyPolicy;
  onAccept: () => Promise<void>;
}) {
  const [declined, setDeclined] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const accept = async () => {
    setBusy(true);
    setError("");
    try {
      await onAccept();
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "The privacy choice could not be saved.",
      );
    } finally {
      setBusy(false);
    }
  };

  if (declined) {
    return (
      <main className="privacy-screen privacy-declined">
        <section className="privacy-declined-card glass-panel">
          <div className="modal-icon">
            <Icon name="shield" />
          </div>
          <h1>SyncSpace is paused</h1>
          <p>
            This device will not advertise itself, look for nearby devices, or
            transfer files while the policy remains unaccepted.
          </p>
          <button className="button primary" onClick={() => setDeclined(false)}>
            Review the policy again
          </button>
        </section>
      </main>
    );
  }

  return (
    <main className="privacy-screen">
      <header className="privacy-header">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            <i />
            <i />
            <i />
          </span>
          <span>SyncSpace</span>
        </div>
        <span className="privacy-version">Policy version {policy.version}</span>
      </header>
      <section className="privacy-layout">
        <div className="privacy-intro">
          <span className="eyebrow">Before SyncSpace uses your network</span>
          <h1>
            {policy.title ||
              "Please review how SyncSpace handles your information"}
          </h1>
          <p>
            {policy.summary ||
              "SyncSpace uses your local network to find devices and transfer files directly between them."}
          </p>
          <div className="privacy-safety-note">
            <Icon name="wifi" />
            <p>
              Discovery and transfers stay off until you accept this policy by
              scrolling to the bottom.
            </p>
          </div>
          {policy.effectiveDate && (
            <p className="privacy-effective-date">
              Effective {formatPolicyDate(policy.effectiveDate)}
            </p>
          )}
        </div>
        <PolicyDocument policy={policy} />
      </section>
      {error && (
        <div className="privacy-error" role="alert">
          {error}
        </div>
      )}
      <footer className="privacy-actions">
        <p>You can read this policy again from Settings at any time.</p>
        <div>
          <button
            className="button ghost"
            disabled={busy}
            onClick={() => setDeclined(true)}
          >
            Decline and keep SyncSpace paused
          </button>
          <button
            className="button primary"
            disabled={busy}
            onClick={() => void accept()}
          >
            <Icon name="check" />
            {busy ? "Saving your choice…" : "Accept and continue"}
          </button>
        </div>
      </footer>
    </main>
  );
}

const fallbackPolicySections: PrivacyPolicySection[] = [
  {
    title: "Finding nearby devices",
    paragraphs: [
      "SyncSpace looks for other SyncSpace devices on the same local network.",
    ],
    items: [
      "Nearby devices can see this device’s name, hostname, platform, app version, local address, availability, and transfer capabilities.",
    ],
  },
  {
    title: "Sending and receiving files",
    paragraphs: [
      "Files move directly between participating devices on the local network. They are not uploaded to a SyncSpace cloud service.",
    ],
    items: [
      "You choose where files are sent.",
      "Incoming transfers need approval.",
      "SyncSpace never opens a received file automatically.",
    ],
  },
  {
    title: "Information stored on this device",
    paragraphs: [
      "SyncSpace stores the settings, device identity, paired-device records, and transfer history needed to provide the service.",
    ],
    items: [
      "File contents are not stored in the SyncSpace database.",
      "Received files are saved only in the folder you choose.",
    ],
  },
  {
    title: "Your controls",
    paragraphs: [
      "You remain in control of discovery, pairing, incoming offers, transfer history, and saved files.",
    ],
    items: [
      "You can stop advertising this device.",
      "You can block or forget paired devices.",
      "You can reject or cancel transfers.",
    ],
  },
];

function PolicyDocument({ policy }: { policy: PrivacyPolicy }) {
  const sections = policy.sections?.length
    ? policy.sections
    : fallbackPolicySections;
  return (
    <article className="privacy-document" aria-label="Privacy policy">
      {sections.map((section) => (
        <section className="privacy-section" key={section.title}>
          <h2>{section.title}</h2>
          {(section.paragraphs || []).map((paragraph) => (
            <p key={paragraph}>{paragraph}</p>
          ))}
          {Boolean(section.items?.length) && (
            <ul>
              {section.items!.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          )}
        </section>
      ))}
    </article>
  );
}

function PrivacyPolicyDialog({
  policy,
  onClose,
}: {
  policy: PrivacyPolicy;
  onClose: () => void;
}) {
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        className="modal privacy-policy-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="privacy-dialog-title"
      >
        <button
          className="modal-close"
          onClick={onClose}
          aria-label="Close privacy policy"
        >
          <Icon name="close" />
        </button>
        <span className="eyebrow">Policy version {policy.version}</span>
        <h2 id="privacy-dialog-title">
          {policy.title || "SyncSpace privacy policy"}
        </h2>
        {policy.summary && <p>{policy.summary}</p>}
        <PolicyDocument policy={policy} />
        <div className="modal-actions">
          <button className="button primary" onClick={onClose}>
            Done
          </button>
        </div>
      </section>
    </div>
  );
}

function HomeView({
  devices,
  trusted,
  transfers,
  onSend,
  onDevices,
}: {
  devices: Device[];
  trusted: TrustedDevice[];
  transfers: Transfer[];
  onSend: () => void;
  onDevices: () => void;
}) {
  const verified = trusted.filter(
    (device) => !device.blocked && !device.identityKeyChanged,
  );
  const completed = transfers.filter(
    (transfer) => transfer.status === "Completed",
  );
  const recent = [...transfers]
    .sort(
      (left, right) => Date.parse(right.updatedAt) - Date.parse(left.updatedAt),
    )
    .slice(0, 3);

  return (
    <section className="home-page">
      <div className="home-hero glass-panel">
        <div className="home-hero-copy">
          <span className="section-kicker">Getting started</span>
          <h2>Share files with another SyncSpace device</h2>
          <p>
            Both devices need to be on the same local network. Pair them once by
            comparing a code, then choose the files you want to send.
          </p>
          <div className="home-actions">
            <button className="button primary" onClick={onSend}>
              <Icon name="send" />
              Choose files to send
            </button>
            <button className="button ghost" onClick={onDevices}>
              <Icon name="devices" />
              Review devices
            </button>
          </div>
        </div>
        <ol className="home-guide" aria-label="How to send files">
          <li>
            <span>1</span>
            <div>
              <strong>Find the other device</strong>
              <p>Keep both devices open on the same Wi-Fi or wired network.</p>
            </div>
          </li>
          <li>
            <span>2</span>
            <div>
              <strong>Pair it safely</strong>
              <p>
                Compare the same code on both screens before you approve the
                device.
              </p>
            </div>
          </li>
          <li>
            <span>3</span>
            <div>
              <strong>Review and send</strong>
              <p>
                Check the destination and file list before the transfer starts.
              </p>
            </div>
          </li>
        </ol>
      </div>

      <div className="home-stats" aria-label="SyncSpace summary">
        <article className="glass-panel">
          <span>Compatible devices nearby</span>
          <strong>{devices.filter((device) => device.online).length}</strong>
          <small>Devices currently available on this network.</small>
        </article>
        <article className="glass-panel">
          <span>Paired devices</span>
          <strong>{verified.length}</strong>
          <small>Devices approved on this SyncSpace installation.</small>
        </article>
        <article className="glass-panel">
          <span>Completed transfers</span>
          <strong>{completed.length}</strong>
          <small>
            Transfers that finished and passed their integrity check.
          </small>
        </article>
      </div>

      <div className="home-lower">
        <section className="glass-panel">
          <div className="section-heading">
            <div>
              <span className="section-kicker">Recent activity</span>
              <h2>Latest transfers</h2>
            </div>
            <button className="text-button" onClick={onSend}>
              Open transfers
            </button>
          </div>
          {recent.length ? (
            <div className="mini-activity">
              {recent.map((transfer) => (
                <div key={transfer.uuid}>
                  <span>
                    <Icon
                      name={
                        transfer.direction === "outbound"
                          ? "arrowUp"
                          : "arrowDown"
                      }
                    />
                  </span>
                  <div>
                    <strong>{transfer.filename}</strong>
                    <small>
                      {transfer.direction === "outbound"
                        ? "Sent to"
                        : "Received from"}{" "}
                      {transfer.deviceName || "another device"}
                    </small>
                  </div>
                  <em>{friendlyTransferStatus(transfer.status)}</em>
                </div>
              ))}
            </div>
          ) : (
            <div className="home-empty">
              Completed and cancelled transfers will appear here.
            </div>
          )}
        </section>
        <aside className="glass-panel privacy-promise">
          <Icon name="wifi" />
          <h3>Files stay on your local network</h3>
          <p>
            SyncSpace does not upload transfer contents to a cloud relay. The
            sending and receiving devices communicate directly.
          </p>
        </aside>
      </div>
    </section>
  );
}

function PairingIntroduction({
  device,
  busy,
  onClose,
  onContinue,
}: {
  device: Device;
  busy: boolean;
  onClose: () => void;
  onContinue: () => void;
}) {
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="pair-title"
      >
        <button
          className="modal-close"
          onClick={onClose}
          aria-label="Close pairing"
        >
          <Icon name="close" />
        </button>
        <div className="modal-icon">
          <Icon name="shield" />
        </div>
        <span className="eyebrow">Pair a device</span>
        <h2 id="pair-title">Pair with {device.deviceName}?</h2>
        <p>
          SyncSpace will show the same short code on both devices. Compare the
          codes before you approve the connection.
        </p>
        <div className="trust-details">
          <span>{device.hostname || device.deviceName}</span>
          <span>{device.platform}</span>
          <span>{device.localIp}</span>
        </div>
        <p className="modal-help">
          Finding a device on the network does not give it permission to send or
          receive files.
        </p>
        <div className="modal-actions">
          <button className="button ghost" disabled={busy} onClick={onClose}>
            Not now
          </button>
          <button
            className="button primary"
            disabled={busy}
            onClick={onContinue}
          >
            <Icon name="shield" />
            {busy ? "Contacting the device…" : "Continue to code check"}
          </button>
        </div>
      </section>
    </div>
  );
}

function PairingModal({
  request,
  onClose,
  onUpdated,
  onPaired,
  onError,
}: {
  request: PairingRequest;
  onClose: () => void;
  onUpdated: (request: PairingRequest) => void;
  onPaired: (device: TrustedDevice) => void;
  onError: (message: string) => void;
}) {
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (
      !request.localConfirmed ||
      request.state === "rejected" ||
      request.state === "paired"
    )
      return;
    const poll = window.setInterval(() => {
      void api
        .refreshPairing(request.requestId)
        .then((decision) => {
          if (decision.trustedDevice) onPaired(decision.trustedDevice);
          else onUpdated(decision.request);
        })
        .catch((error) =>
          onError(
            error instanceof Error
              ? error.message
              : "SyncSpace could not refresh the pairing request.",
          ),
        );
    }, 1500);
    return () => window.clearInterval(poll);
  }, [
    onError,
    onPaired,
    onUpdated,
    request.localConfirmed,
    request.requestId,
    request.state,
  ]);

  const confirm = async () => {
    setBusy(true);
    try {
      const decision = await api.confirmPairing(request.requestId);
      if (decision.trustedDevice) onPaired(decision.trustedDevice);
      else onUpdated(decision.request);
    } catch (error) {
      onError(
        error instanceof Error
          ? error.message
          : "SyncSpace could not confirm the pairing code.",
      );
    } finally {
      setBusy(false);
    }
  };

  const reject = async () => {
    setBusy(true);
    try {
      await api.rejectPairing(request.requestId);
      onClose();
    } catch (error) {
      onError(
        error instanceof Error
          ? error.message
          : "SyncSpace could not reject the pairing request.",
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="modal-backdrop">
      <section
        className="modal pairing-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="verify-title"
      >
        <button
          className="modal-close"
          onClick={onClose}
          aria-label="Close pairing"
        >
          <Icon name="close" />
        </button>
        <div className="modal-icon">
          <Icon name="shield" />
        </div>
        <span className="eyebrow">Check both screens</span>
        <h2 id="verify-title">Compare this code with {request.deviceName}</h2>
        <p>
          The person using the other device should see the same code. If any
          digit is different, choose “Codes do not match”.
        </p>
        <div
          className="verification-code"
          aria-label={`Verification code ${request.verificationCode}`}
        >
          {request.verificationCode}
        </div>
        <details className="technical-details">
          <summary>Show technical identity details</summary>
          <div className="fingerprint-box">
            <span>Device identity fingerprint</span>
            <code>{request.fingerprint}</code>
          </div>
        </details>
        {request.localConfirmed ? (
          <div className="pairing-wait" role="status">
            <span className="loading-pulse" />
            <div>
              <strong>Your code is confirmed</strong>
              <small>Waiting for the other device to confirm its code.</small>
            </div>
          </div>
        ) : (
          <div className="modal-actions">
            <button
              className="button ghost"
              disabled={busy}
              onClick={() => void reject()}
            >
              Codes do not match
            </button>
            <button
              className="button primary"
              disabled={busy}
              onClick={() => void confirm()}
            >
              <Icon name="check" />
              {busy ? "Confirming the code…" : "Codes match"}
            </button>
          </div>
        )}
      </section>
    </div>
  );
}

export function SendReviewModal({
  files,
  device,
  busy,
  onAddFiles,
  onAddFolder,
  onRemove,
  onClear,
  onCancel,
  onConfirm,
}: {
  files: LocalFile[];
  device?: Device;
  busy: boolean;
  onAddFiles: () => void;
  onAddFolder: () => void;
  onRemove: (index: number) => void;
  onClear: () => void;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const totalSize = files.reduce((sum, item) => sum + item.file.size, 0);
  const riskyFiles = files.filter((item) =>
    isPotentiallyExecutable(item.relativePath),
  );
  const folderRoots = topLevelRoots(files).filter((root) =>
    files.some((item) => item.relativePath.startsWith(`${root}/`)),
  );

  return (
    <div className="modal-backdrop">
      <section
        className="modal send-review-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="send-review-title"
      >
        <button
          className="modal-close"
          disabled={busy}
          onClick={onCancel}
          aria-label="Close transfer review"
        >
          <Icon name="close" />
        </button>
        <span className="eyebrow">Review before sending</span>
        <h2 id="send-review-title">Check these files and the destination</h2>
        <p>Nothing will leave this device until you choose “Send files”.</p>

        <div className="review-destination">
          <span className="large-device-icon">
            <PlatformGlyph platform={device?.platform || ""} />
          </span>
          <div>
            <span>Send to</span>
            <strong>{device?.deviceName || "Device unavailable"}</strong>
            <small>
              {device
                ? `${device.hostname || "Hostname unavailable"} · ${friendlyPlatform(device.platform)}`
                : "Reconnect this device before sending."}
            </small>
          </div>
        </div>

        <div className="review-summary" aria-label="Selected file summary">
          <div>
            <span>Files</span>
            <strong>{files.length}</strong>
          </div>
          <div>
            <span>Total size</span>
            <strong>{formatBytes(totalSize)}</strong>
          </div>
        </div>

        {folderRoots.length > 0 && (
          <div className="review-folder-roots">
            <span>Folders included</span>
            <p>{folderRoots.join(", ")}</p>
          </div>
        )}

        {riskyFiles.length > 0 && (
          <div className="content-warning" role="note">
            <Icon name="shield" />
            <p>
              <strong>
                This selection contains executable or script files.
              </strong>
              Only send them when the person receiving them expects and trusts
              the files.
            </p>
          </div>
        )}

        <div className="review-list-actions">
          <div>
            <button
              className="button ghost"
              disabled={busy}
              onClick={onAddFiles}
            >
              <Icon name="file" />
              Add more files
            </button>
            <button
              className="button ghost"
              disabled={busy}
              onClick={onAddFolder}
            >
              <Icon name="folder" />
              Add a folder
            </button>
          </div>
          <button
            className="text-button danger-text"
            disabled={busy || !files.length}
            onClick={onClear}
          >
            Clear all
          </button>
        </div>

        {files.length ? (
          <div
            className="review-file-list"
            tabIndex={0}
            aria-label="Files selected for transfer"
          >
            <ul>
              {files.map((item, index) => (
                <li key={`${item.relativePath}-${index}`}>
                  <Icon name="file" />
                  <span title={item.relativePath}>{item.relativePath}</span>
                  <small>{formatBytes(item.file.size)}</small>
                  <button
                    className="review-file-remove"
                    disabled={busy}
                    aria-label={`Remove ${item.relativePath}`}
                    onClick={() => onRemove(index)}
                  >
                    <Icon name="close" />
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ) : (
          <div className="review-empty" role="status">
            <Icon name="file" />
            <p>No files are selected. Add files or a folder to continue.</p>
          </div>
        )}

        <div className="modal-actions">
          <button className="button ghost" disabled={busy} onClick={onCancel}>
            Go back
          </button>
          <button
            className="button primary"
            disabled={busy || !device || !files.length}
            onClick={onConfirm}
          >
            <Icon name="send" />
            {busy
              ? "Preparing the files…"
              : `Send ${files.length === 1 ? "file" : `${files.length} files`}`}
          </button>
        </div>
      </section>
    </div>
  );
}

function SettingsView({
  settings,
  localDevice,
  policy,
  onViewPrivacy,
  onChanged,
  onLocalDeviceChanged,
  onToast,
}: {
  settings: Settings;
  localDevice: Device | null;
  policy: PrivacyPolicy;
  onViewPrivacy: () => void;
  onChanged: (settings: Settings) => void;
  onLocalDeviceChanged: (device: Device) => void;
  onToast: (
    tone: "success" | "error" | "info",
    title: string,
    message: string,
  ) => void;
}) {
  const [draft, setDraft] = useState(settings);
  const [busy, setBusy] = useState(false);

  useEffect(() => setDraft(settings), [settings]);

  const update = <K extends keyof Settings>(key: K, value: Settings[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const save = async () => {
    const deviceName = (draft.deviceName || "").trim();
    if (!deviceName) {
      onToast(
        "error",
        "Enter a device name",
        "Nearby devices need a clear name so people can recognise this device.",
      );
      return;
    }
    setBusy(true);
    try {
      const saved = await api.updateSettings({
        ...draft,
        deviceName,
      });
      onChanged(saved);
      if (localDevice)
        onLocalDeviceChanged({ ...localDevice, deviceName: saved.deviceName });
      void api
        .self()
        .then(onLocalDeviceChanged)
        .catch(() => undefined);
      if (
        saved.notificationsEnabled &&
        "Notification" in window &&
        Notification.permission === "default"
      ) {
        void Notification.requestPermission();
      }
      onToast(
        "success",
        "Settings saved",
        "SyncSpace is now using these choices on this device.",
      );
    } catch (error) {
      onToast(
        "error",
        "Settings could not be saved",
        error instanceof Error
          ? error.message
          : "Check the values and try again.",
      );
    } finally {
      setBusy(false);
    }
  };

  const reset = async () => {
    if (
      !window.confirm("Restore the default SyncSpace settings on this device?")
    )
      return;
    setBusy(true);
    try {
      const defaults = await api.resetSettings();
      setDraft(defaults);
      onChanged(defaults);
      onToast(
        "info",
        "Default settings restored",
        "Review the restored choices before you continue using SyncSpace.",
      );
    } catch (error) {
      onToast(
        "error",
        "Settings could not be reset",
        error instanceof Error ? error.message : "Try again in a moment.",
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="settings-page">
      <div className="settings-grid">
        <article className="glass-panel settings-card settings-card-wide">
          <div>
            <span className="section-kicker">This device</span>
            <h2>Name and network availability</h2>
            <p className="settings-card-description">
              Choose what nearby SyncSpace users can see and whether they can
              contact this device.
            </p>
          </div>
          <label>
            <span>Device name</span>
            <input
              value={draft.deviceName || ""}
              maxLength={80}
              onChange={(event) => update("deviceName", event.target.value)}
              placeholder={localDevice?.hostname || "My computer"}
            />
            <small>
              This is the friendly name people will see when they find this
              device.
            </small>
          </label>
          <label className="toggle-row">
            <span>
              <strong>Use local-network discovery</strong>
              <small>
                Turn this off to stop finding nearby devices and stop
                advertising this device until you enable it again.
              </small>
            </span>
            <input
              type="checkbox"
              checked={draft.discoverable ?? false}
              onChange={(event) => update("discoverable", event.target.checked)}
            />
          </label>
          <label className="toggle-row">
            <span>
              <strong>Allow new incoming transfer offers</strong>
              <small>
                When this is off, other devices receive a clear rejection before
                any file data is sent.
              </small>
            </span>
            <input
              type="checkbox"
              checked={draft.incomingTransfersEnabled ?? false}
              onChange={(event) =>
                update("incomingTransfersEnabled", event.target.checked)
              }
            />
          </label>
        </article>

        <article className="glass-panel settings-card">
          <div>
            <span className="section-kicker">Appearance</span>
            <h2>Theme and movement</h2>
          </div>
          <label>
            <span>Colour mode</span>
            <select
              value={draft.appearance}
              onChange={(event) =>
                update(
                  "appearance",
                  event.target.value as Settings["appearance"],
                )
              }
            >
              <option value="system">Follow this device</option>
              <option value="dark">Dark</option>
              <option value="light">Light</option>
            </select>
          </label>
          <label className="toggle-row">
            <span>
              <strong>Reduce movement</strong>
              <small>
                Use simpler transitions and do not show the completion
                celebration.
              </small>
            </span>
            <input
              type="checkbox"
              checked={draft.reducedMotion}
              onChange={(event) =>
                update("reducedMotion", event.target.checked)
              }
            />
          </label>
        </article>

        <article className="glass-panel settings-card">
          <div>
            <span className="section-kicker">Incoming files</span>
            <h2>Save location and name conflicts</h2>
          </div>
          <label>
            <span>Default save folder</span>
            <input
              value={draft.defaultDownloadDirectory}
              onChange={(event) =>
                update("defaultDownloadDirectory", event.target.value)
              }
              placeholder="Enter an absolute Downloads folder path"
            />
            <small>
              You can choose a different folder whenever you accept an incoming
              transfer.
            </small>
          </label>
          <label>
            <span>If a file already has the same name</span>
            <select
              value={draft.conflictPolicy}
              onChange={(event) =>
                update("conflictPolicy", event.target.value as ConflictPolicy)
              }
            >
              <option value="rename">Keep both files</option>
              <option value="prompt">Ask before deciding</option>
              <option value="overwrite">Replace the existing file</option>
            </select>
          </label>
        </article>

        <article className="glass-panel settings-card">
          <div>
            <span className="section-kicker">Notifications</span>
            <h2>Transfer completion</h2>
          </div>
          <label className="toggle-row">
            <span>
              <strong>Show a notification when a transfer finishes</strong>
              <small>
                Completion notifications can include the transferred filename
                and may appear on the lock screen. Leave this off on shared
                devices.
              </small>
            </span>
            <input
              type="checkbox"
              checked={draft.notificationsEnabled}
              onChange={(event) =>
                update("notificationsEnabled", event.target.checked)
              }
            />
          </label>
          <div className="settings-note">
            <Icon name="shield" />
            <p>
              Diagnostic exports leave out file contents and secret keys, but
              may include device names, IP addresses, filenames, local paths,
              and recent errors. Review an export before sharing it.
            </p>
          </div>
        </article>

        <article className="glass-panel settings-card privacy-settings-card">
          <div>
            <span className="section-kicker">Privacy</span>
            <h2>Your accepted policy</h2>
            <p className="settings-card-description">
              Review what SyncSpace shares on your local network and what it
              stores on this device.
            </p>
          </div>
          <dl className="settings-details">
            <div>
              <dt>Policy version</dt>
              <dd>{settings.privacyPolicyVersion || policy.version}</dd>
            </div>
            <div>
              <dt>Accepted</dt>
              <dd>
                {formatAcceptedAt(
                  settings.privacyAcceptedAt || policy.acceptedAt,
                )}
              </dd>
            </div>
            <div>
              <dt>Local device ID</dt>
              <dd title={localDevice?.deviceId}>
                {localDevice?.deviceId || "Unavailable"}
              </dd>
            </div>
          </dl>
          <div className="privacy-settings-actions">
            <button className="button ghost" onClick={onViewPrivacy}>
              Read the privacy policy
            </button>
            <a
              className="button ghost"
              href="/api/v1/diagnostics/export"
              download
            >
              <Icon name="arrowDown" />
              Export diagnostics
            </a>
          </div>
        </article>
      </div>

      <div className="settings-actions">
        <button
          className="button ghost"
          disabled={busy}
          onClick={() => void reset()}
        >
          Restore default settings
        </button>
        <button
          className="button primary"
          disabled={busy}
          onClick={() => void save()}
        >
          <Icon name="check" />
          {busy ? "Saving settings…" : "Save settings"}
        </button>
      </div>
    </section>
  );
}

function NavButton({
  icon,
  label,
  active,
  badge,
  onClick,
}: {
  icon: IconName;
  label: string;
  active: boolean;
  badge?: number;
  onClick: () => void;
}) {
  return (
    <button
      className={active ? "active" : ""}
      aria-current={active ? "page" : undefined}
      onClick={onClick}
    >
      <Icon name={icon} />
      <span>{label}</span>
      {badge !== undefined && <em aria-label={`${badge} items`}>{badge}</em>}
    </button>
  );
}

interface TransferViewProps {
  devices: Device[];
  discoveryEnabled: boolean;
  trustedIDs: Set<string>;
  selectedDevice: string;
  onSelect: (id: string) => void;
  active: Transfer[];
  activeCount: number;
  totalSpeed: number;
  onBrowseFiles: () => void;
  onBrowseFolder: () => void;
  onPair: (device: Device) => void;
  onRefresh: () => void;
  onOpenSettings: () => void;
  onChanged: (transfer: Transfer) => void;
  onError: (message: string) => void;
  defaultDestination: string;
  defaultConflictPolicy: ConflictPolicy;
}

export function TransferView(props: TransferViewProps) {
  const selected = props.devices.find(
    (device) => device.deviceId === props.selectedDevice,
  );
  const canChooseFiles = Boolean(
    selected && props.trustedIDs.has(selected.deviceId),
  );

  return (
    <div className="page-grid transfer-page">
      <section className="send-panel glass-panel">
        <div className="section-heading transfer-step-heading">
          <div className="step-title">
            <span className="step-number" aria-hidden="true">
              1
            </span>
            <div>
              <span className="section-kicker">Choose a destination</span>
              <h2>Select a paired device</h2>
              <p>Only devices marked Ready can be selected for a transfer.</p>
            </div>
          </div>
          <button className="text-button" onClick={props.onRefresh}>
            <Icon name="refresh" />
            Refresh devices
          </button>
        </div>

        <div className="device-strip" aria-label="Compatible nearby devices">
          {props.devices.length ? (
            props.devices.map((device) => {
              const trusted = props.trustedIDs.has(device.deviceId);
              const selectedDevice = props.selectedDevice === device.deviceId;
              return (
                <article
                  key={device.deviceId}
                  className={`device-orb-card ${selectedDevice ? "selected" : ""} ${trusted ? "ready" : "needs-pairing"}`}
                >
                  <button
                    className="device-choice-button"
                    disabled={!trusted}
                    aria-pressed={selectedDevice}
                    onClick={() => props.onSelect(device.deviceId)}
                  >
                    <span className="device-orb">
                      <PlatformGlyph platform={device.platform} />
                      <i className="online-dot" />
                    </span>
                    <strong>{device.deviceName}</strong>
                    <small>{device.hostname || device.platform}</small>
                  </button>
                  {trusted ? (
                    <span className="device-ready-label">
                      <Icon name="check" />
                      Ready
                    </span>
                  ) : (
                    <button
                      className="pair-device-button"
                      onClick={() => props.onPair(device)}
                    >
                      Pair device
                    </button>
                  )}
                </article>
              );
            })
          ) : (
            <div className="empty-devices">
              <div className="radar small" aria-hidden="true">
                <span />
                <span />
                <i />
              </div>
              <div>
                <strong>
                  {props.discoveryEnabled
                    ? "No compatible device is available yet"
                    : "Local-network discovery is off"}
                </strong>
                <span>
                  {props.discoveryEnabled
                    ? "Keep both devices open on the same local network, then refresh this list."
                    : "Enable discovery in Settings before SyncSpace can find or advertise devices."}
                </span>
              </div>
              {!props.discoveryEnabled && (
                <button className="button ghost" onClick={props.onOpenSettings}>
                  Open settings
                </button>
              )}
            </div>
          )}
        </div>

        <div
          className={`selected-device-summary ${selected ? "ready" : ""}`}
          role="status"
        >
          <Icon name={selected ? "check" : "devices"} />
          <div>
            <strong>
              {selected
                ? `${selected.deviceName} is selected`
                : "Choose a paired device to continue"}
            </strong>
            <span>
              {selected
                ? `Files will be sent to ${selected.hostname || selected.localIp} after you review them.`
                : "If the device is new, pair it and compare the code shown on both screens."}
            </span>
          </div>
        </div>

        <div
          className={`drop-zone ${canChooseFiles ? "" : "disabled"}`}
          aria-disabled={!canChooseFiles}
        >
          <span className="step-number" aria-hidden="true">
            2
          </span>
          <span className="drop-icon">
            <Icon name="plus" />
          </span>
          <h3>Choose the files or folder you want to send</h3>
          <p>
            You will review the complete file list and total size before the
            transfer begins.
          </p>
          <div className="drop-actions">
            <button
              className="button primary"
              disabled={!canChooseFiles}
              onClick={props.onBrowseFiles}
            >
              <Icon name="file" />
              Choose files
            </button>
            <button
              className="button ghost"
              disabled={!canChooseFiles}
              onClick={props.onBrowseFolder}
            >
              <Icon name="folder" />
              Choose a folder
            </button>
          </div>
        </div>

        <div className="send-options">
          <p>
            The receiving device decides how to handle an existing file with the
            same name.
          </p>
          <span className="secure-note">
            <Icon name="shield" />
            Completed files are checked with SHA-256.
          </span>
        </div>
      </section>

      <aside className="pulse-panel transfer-summary glass-panel">
        <span className="section-kicker">Current transfer status</span>
        <h2>
          {props.activeCount
            ? `${props.activeCount} ${props.activeCount === 1 ? "transfer is" : "transfers are"} moving`
            : "No files are moving right now"}
        </h2>
        <p>
          Incoming offers and queued, paused, or failed transfers remain listed
          below until you act on them.
        </p>
        <dl className="transfer-summary-values">
          <div>
            <dt>Currently moving</dt>
            <dd>{props.activeCount}</dd>
          </div>
          <div>
            <dt>Listed below</dt>
            <dd>{props.active.length}</dd>
          </div>
          <div>
            <dt>Combined speed</dt>
            <dd>
              {props.totalSpeed
                ? `${formatBytes(props.totalSpeed)}/s`
                : "No active data"}
            </dd>
          </div>
        </dl>
        <div className="privacy-card">
          <Icon name="wifi" />
          <div>
            <strong>Transfers use your local network</strong>
            <span>
              SyncSpace does not create a cloud copy of the transferred files.
            </span>
          </div>
        </div>
      </aside>

      <section className="queue-section">
        <div className="section-heading">
          <div className="step-title">
            <span className="step-number" aria-hidden="true">
              3
            </span>
            <div>
              <span className="section-kicker">Follow progress</span>
              <h2>Current transfers and incoming offers</h2>
            </div>
          </div>
          <span className="queue-count">
            {props.active.length} {props.active.length === 1 ? "item" : "items"}
          </span>
        </div>
        <div className="transfer-list">
          {props.active.length ? (
            props.active.map((transfer) => (
              <TransferCard
                key={transfer.uuid}
                transfer={transfer}
                onChanged={props.onChanged}
                onError={props.onError}
                defaultDestination={props.defaultDestination}
                defaultConflictPolicy={props.defaultConflictPolicy}
              />
            ))
          ) : (
            <div className="empty-queue">
              <span>
                <Icon name="send" />
              </span>
              <h3>There are no current transfers</h3>
              <p>
                Choose a paired device and select files when you are ready to
                send something.
              </p>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

export function DevicesView({
  devices,
  discoveryEnabled,
  localDevice,
  trusted,
  onSelect,
  onPair,
  onRefresh,
  onOpenSettings,
  onTrustedChanged,
  onForgot,
  onError,
}: {
  devices: Device[];
  discoveryEnabled: boolean;
  localDevice: Device | null;
  trusted: TrustedDevice[];
  onSelect: (device: Device) => void;
  onPair: (device: Device) => void;
  onRefresh: () => void;
  onOpenSettings: () => void;
  onTrustedChanged: (device: TrustedDevice) => void;
  onForgot: (id: string) => void;
  onError: (message: string) => void;
}) {
  const byID = new Map(trusted.map((device) => [device.deviceId, device]));
  const sortedDevices = [...devices].sort(
    (left, right) => Number(right.online) - Number(left.online),
  );

  const updateBlock = async (device: TrustedDevice) => {
    try {
      onTrustedChanged(
        await api.setDeviceBlocked(device.deviceId, !device.blocked),
      );
    } catch (error) {
      onError(
        error instanceof Error
          ? error.message
          : "SyncSpace could not change access for this device.",
      );
    }
  };

  const forget = async (device: TrustedDevice) => {
    if (
      !window.confirm(
        `Forget ${device.deviceName}? You will need to compare a new pairing code before using it again.`,
      )
    )
      return;
    try {
      await api.forgetDevice(device.deviceId);
      onForgot(device.deviceId);
    } catch (error) {
      onError(
        error instanceof Error
          ? error.message
          : "SyncSpace could not forget this device.",
      );
    }
  };

  return (
    <section className="devices-page">
      <div className="section-toolbar">
        <p>
          {!discoveryEnabled
            ? "Local-network discovery is off, so SyncSpace is not finding or advertising devices."
            : devices.length
              ? `${devices.length} ${devices.length === 1 ? "device is" : "devices are"} known to SyncSpace.`
              : "No other SyncSpace device has been found yet."}
        </p>
        {discoveryEnabled ? (
          <button className="button ghost" onClick={onRefresh}>
            <Icon name="refresh" />
            Refresh devices
          </button>
        ) : (
          <button className="button ghost" onClick={onOpenSettings}>
            Open settings
          </button>
        )}
      </div>
      <div className="device-grid">
        {localDevice && (
          <article className="device-card local-device-card glass-panel">
            <div className="device-card-top">
              <span className="large-device-icon">
                <PlatformGlyph platform={localDevice.platform} />
              </span>
              <span className="presence online">This device</span>
            </div>
            <h2>{localDevice.deviceName}</h2>
            <p>
              {localDevice.hostname || "Hostname unavailable"} ·{" "}
              {friendlyPlatform(localDevice.platform)}
            </p>
            <div className="capability-grid">
              <div>
                <span>Network address</span>
                <strong>{localDevice.localIp}</strong>
              </div>
              <div>
                <span>SyncSpace version</span>
                <strong>{localDevice.appVersion}</strong>
              </div>
              <div>
                <span>Discovery</span>
                <strong>{discoveryEnabled ? "On" : "Off"}</strong>
              </div>
              <div>
                <span>Device ID</span>
                <strong title={localDevice.deviceId}>Saved locally</strong>
              </div>
            </div>
            <div className="device-card-actions">
              <span className="trusted-label">
                <Icon name="shield" />
                Managed on this device
              </span>
              <button className="button ghost" onClick={onOpenSettings}>
                Open settings
              </button>
            </div>
          </article>
        )}
        {sortedDevices.map((device) => {
          const trust = byID.get(device.deviceId);
          const incompatible =
            !device.transferCapability ||
            device.supportedProtocolVersion !== CURRENT_PROTOCOL_VERSION;
          return (
            <article className="device-card glass-panel" key={device.deviceId}>
              <div className="device-card-top">
                <span className="large-device-icon">
                  <PlatformGlyph platform={device.platform} />
                </span>
                <span className={`presence ${device.online ? "online" : ""}`}>
                  {device.online ? "Online now" : "Offline"}
                </span>
              </div>
              <h2>{trust?.localName || device.deviceName}</h2>
              <p>
                {device.hostname || "Hostname unavailable"} ·{" "}
                {friendlyPlatform(device.platform)}
              </p>

              {incompatible && (
                <div className="device-warning incompatible" role="status">
                  <strong>
                    This device needs a compatible SyncSpace version.
                  </strong>
                  <span>
                    Update SyncSpace on the other device before pairing or
                    transferring files.
                  </span>
                </div>
              )}

              {trust?.identityKeyChanged && (
                <div className="device-warning" role="alert">
                  <strong>This device’s identity changed.</strong>
                  <span>
                    Transfers stay blocked until you forget the old record and
                    pair the device again.
                  </span>
                </div>
              )}

              <div className="capability-grid">
                <div>
                  <span>Network address</span>
                  <strong>{device.localIp}</strong>
                </div>
                <div>
                  <span>Available storage</span>
                  <strong>{formatBytes(device.availableStorage)}</strong>
                </div>
                <div>
                  <span>Last seen</span>
                  <strong>{formatLastSeen(device.lastSeen)}</strong>
                </div>
                <div>
                  <span>Transfer support</span>
                  <strong>
                    {incompatible ? "Not compatible" : "Compatible"}
                  </strong>
                </div>
              </div>

              <details className="technical-details device-technical-details">
                <summary>Show technical details</summary>
                <dl>
                  <div>
                    <dt>SyncSpace version</dt>
                    <dd>{device.appVersion}</dd>
                  </div>
                  <div>
                    <dt>Protocol</dt>
                    <dd>Version {device.supportedProtocolVersion}</dd>
                  </div>
                  <div>
                    <dt>Maximum chunk size</dt>
                    <dd>{formatBytes(device.maximumChunkSize)}</dd>
                  </div>
                </dl>
                {trust && (
                  <div
                    className={`fingerprint-box ${trust.identityKeyChanged ? "warning" : ""}`}
                  >
                    <span>Saved device identity fingerprint</span>
                    <code>{trust.fingerprint}</code>
                  </div>
                )}
              </details>

              <div className="device-card-actions">
                {trust ? (
                  <>
                    <span
                      className={
                        trust.blocked || trust.identityKeyChanged
                          ? "untrusted-label"
                          : "trusted-label"
                      }
                    >
                      <Icon name="shield" />
                      {trust.identityKeyChanged
                        ? "Pair again"
                        : trust.blocked
                          ? "Blocked"
                          : "Paired"}
                    </span>
                    <button
                      className="button ghost"
                      onClick={() => void updateBlock(trust)}
                    >
                      {trust.blocked ? "Allow again" : "Block"}
                    </button>
                    <button
                      className="button ghost danger-text"
                      onClick={() => void forget(trust)}
                    >
                      Forget
                    </button>
                    <button
                      className="button primary"
                      disabled={
                        !device.online ||
                        incompatible ||
                        trust.blocked ||
                        trust.identityKeyChanged
                      }
                      onClick={() => onSelect(device)}
                    >
                      Send files
                    </button>
                  </>
                ) : (
                  <>
                    <span className="untrusted-label">
                      {incompatible ? "Update needed" : "Not paired yet"}
                    </span>
                    <button
                      className="button primary"
                      disabled={
                        !device.online ||
                        incompatible ||
                        !device.pairingAvailable
                      }
                      onClick={() => onPair(device)}
                    >
                      {incompatible ? "Pairing unavailable" : "Pair device"}
                    </button>
                  </>
                )}
              </div>
            </article>
          );
        })}
        {!devices.length && (
          <div className="large-empty">
            <div className="radar" aria-hidden="true">
              <span />
              <span />
              <i />
            </div>
            <h2>No nearby devices have been found</h2>
            <p>
              {discoveryEnabled
                ? "Open SyncSpace on the other device, connect both devices to the same local network, and then refresh this page."
                : "Enable local-network discovery in Settings before SyncSpace can find other devices."}
            </p>
            <button
              className="button primary"
              onClick={discoveryEnabled ? onRefresh : onOpenSettings}
            >
              <Icon name={discoveryEnabled ? "refresh" : "settings"} />
              {discoveryEnabled ? "Refresh devices" : "Open settings"}
            </button>
          </div>
        )}
      </div>
    </section>
  );
}

export function HistoryView({
  transfers,
  onChanged,
  onError,
  onCleared,
  defaultDestination,
  defaultConflictPolicy,
}: {
  transfers: Transfer[];
  onChanged: (transfer: Transfer) => void;
  onError: (message: string) => void;
  onCleared: () => void;
  defaultDestination: string;
  defaultConflictPolicy: ConflictPolicy;
}) {
  const [filter, setFilter] = useState<
    "all" | "sent" | "received" | "completed" | "failed"
  >("all");
  const filteredTransfers = transfers.filter((transfer) => {
    if (filter === "sent") return transfer.direction === "outbound";
    if (filter === "received") return transfer.direction === "inbound";
    if (filter === "completed") return transfer.status === "Completed";
    if (filter === "failed") return transfer.status === "Failed";
    return true;
  });

  const clear = async () => {
    if (
      !window.confirm(
        "Clear completed, cancelled, and failed transfers from this device? This does not delete any transferred files.",
      )
    )
      return;
    try {
      await api.clearHistory();
      onCleared();
    } catch (error) {
      onError(
        error instanceof Error
          ? error.message
          : "SyncSpace could not clear the transfer history.",
      );
    }
  };

  return (
    <section className="history-page">
      <div className="section-heading history-heading">
        <div>
          <span className="section-kicker">Stored on this device</span>
          <h2>Transfer history</h2>
          <p>
            Clearing this list does not remove files that were sent or received.
          </p>
        </div>
        {transfers.length > 0 && (
          <button className="button ghost" onClick={() => void clear()}>
            Clear history
          </button>
        )}
      </div>
      <div
        className="history-filters"
        role="group"
        aria-label="Filter transfer history"
      >
        {(
          [
            ["all", "All"],
            ["sent", "Sent"],
            ["received", "Received"],
            ["completed", "Completed"],
            ["failed", "Failed"],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            className={filter === value ? "active" : ""}
            aria-pressed={filter === value}
            onClick={() => setFilter(value)}
          >
            {label}
          </button>
        ))}
        <span>
          {filteredTransfers.length}{" "}
          {filteredTransfers.length === 1 ? "record" : "records"}
        </span>
      </div>
      <div className="transfer-list">
        {filteredTransfers.length ? (
          filteredTransfers.map((transfer) => (
            <TransferCard
              key={transfer.uuid}
              transfer={transfer}
              onChanged={onChanged}
              onError={onError}
              defaultDestination={defaultDestination}
              defaultConflictPolicy={defaultConflictPolicy}
            />
          ))
        ) : transfers.length ? (
          <div className="large-empty history-filter-empty">
            <span className="empty-history-icon">
              <Icon name="history" />
            </span>
            <h2>No transfers match this filter</h2>
            <p>Choose another filter to review the rest of the history.</p>
          </div>
        ) : (
          <div className="large-empty">
            <span className="empty-history-icon">
              <Icon name="history" />
            </span>
            <h2>There is no transfer history yet</h2>
            <p>
              Completed, cancelled, and failed transfers will be listed here
              after they occur.
            </p>
          </div>
        )}
      </div>
    </section>
  );
}

function UploadNotice({ upload }: { upload: UploadProgress }) {
  const percent = Math.round(
    upload.totalBytes ? (upload.completedBytes / upload.totalBytes) * 100 : 0,
  );
  return (
    <div className="upload-float" role="status" aria-live="polite">
      <div
        className="upload-ring"
        role="progressbar"
        aria-label="Preparing selected files"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
        style={{ "--progress": `${percent * 3.6}deg` } as React.CSSProperties}
      >
        <Icon name="arrowUp" />
      </div>
      <div>
        <strong>Preparing the files on this device</strong>
        <span>{upload.currentName}</span>
        <small>
          File {Math.min(upload.completedFiles + 1, upload.totalFiles)} of{" "}
          {upload.totalFiles} · {percent}%
        </small>
      </div>
    </div>
  );
}

function PlatformGlyph({ platform }: { platform: string }) {
  const value = platform.toLowerCase();
  const glyph = value.includes("android")
    ? "A"
    : value.includes("ios") || value.includes("mac")
      ? "⌘"
      : value.includes("linux")
        ? "L"
        : value.includes("windows")
          ? "W"
          : "D";
  return (
    <span className="platform-glyph" aria-hidden="true">
      {glyph}
    </span>
  );
}

function Celebration() {
  return (
    <div className="celebration" aria-hidden="true">
      {Array.from({ length: 18 }, (_, index) => (
        <i key={index} style={{ "--i": index } as React.CSSProperties} />
      ))}
    </div>
  );
}

function filesFromList(list: FileList | null): LocalFile[] {
  return Array.from(list || [])
    .map((file) => ({
      file,
      relativePath: cleanRelativePath(
        (file as File & { webkitRelativePath?: string }).webkitRelativePath ||
          file.name,
      ),
    }))
    .filter((item) => item.relativePath);
}

function mergeLocalFiles(
  existing: LocalFile[],
  added: LocalFile[],
): LocalFile[] {
  const byPath = new Map(existing.map((item) => [item.relativePath, item]));
  for (const item of added) byPath.set(item.relativePath, item);
  return [...byPath.values()];
}

async function filesFromDrop(data: DataTransfer): Promise<LocalFile[]> {
  const items = Array.from(data.items);
  const entries = items
    .map((item) => item.webkitGetAsEntry())
    .filter((entry): entry is FileSystemEntry => entry !== null);
  if (!entries.length) return filesFromList(data.files);
  const output: LocalFile[] = [];
  for (const entry of entries) await walkEntry(entry, "", output);
  return output;
}

async function walkEntry(
  entry: FileSystemEntry,
  parent: string,
  output: LocalFile[],
): Promise<void> {
  const relativePath = cleanRelativePath(
    parent ? `${parent}/${entry.name}` : entry.name,
  );
  if (entry.isFile) {
    const file = await new Promise<File>((resolve, reject) =>
      (entry as FileSystemFileEntry).file(resolve, reject),
    );
    output.push({ file, relativePath });
    return;
  }
  const reader = (entry as FileSystemDirectoryEntry).createReader();
  const children: FileSystemEntry[] = [];
  while (true) {
    const batch = await new Promise<FileSystemEntry[]>((resolve, reject) =>
      reader.readEntries(resolve, reject),
    );
    if (!batch.length) break;
    children.push(...batch);
  }
  for (const child of children) await walkEntry(child, relativePath, output);
}

function friendlyTransferStatus(status: Transfer["status"]): string {
  const labels: Record<Transfer["status"], string> = {
    Queued: "Waiting",
    Preparing: "Preparing",
    Connecting: "Connecting",
    Negotiating: "Agreeing transfer details",
    Sending: "Sending",
    Receiving: "Receiving",
    Paused: "Paused",
    Resuming: "Resuming",
    Completed: "Completed",
    Cancelled: "Cancelled",
    Failed: "Needs attention",
    Verifying: "Checking the file",
  };
  return labels[status];
}

function friendlyPlatform(platform: string): string {
  if (!platform) return "Platform unavailable";
  return platform.charAt(0).toUpperCase() + platform.slice(1);
}

function formatLastSeen(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown";
  const seconds = Math.max(0, Math.round((Date.now() - date.getTime()) / 1000));
  if (seconds < 60) return "Just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} hr ago`;
  return date.toLocaleDateString();
}

function formatAcceptedAt(value?: string): string {
  if (!value) return "Not recorded";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function formatPolicyDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString();
}

function initials(value: string): string {
  const parts = value.trim().split(/\s+/).filter(Boolean);
  if (!parts.length) return "SS";
  return parts
    .slice(0, 2)
    .map((part) => part.charAt(0).toUpperCase())
    .join("");
}
