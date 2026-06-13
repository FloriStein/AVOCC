import { useState } from "react";
import { useSystemState } from "@/hooks/useSystemState";
import { useSession } from "@/hooks/useSession";
import { useTelemetry } from "@/hooks/useTelemetry";
import { useVehicleAck } from "@/hooks/useVehicleAck";
import { SafeModeOverlay } from "@/components/SafeModeOverlay";
import { SafetyPanel } from "@/components/SafetyPanel";
import { ConnectionPanel } from "@/components/ConnectionPanel";
import { VideoPanel } from "@/components/VideoPanel";
import { ControlPanel } from "@/components/ControlPanel";
import { InputIndicatorPanel } from "@/components/InputIndicatorPanel";
import { StreamSenderPanel } from "@/components/StreamSenderPanel";
import LoginPanel from "@/components/LoginPanel";
import UserManagementPanel from "@/components/UserManagementPanel";
import { parseTokenRole } from "@/lib/api-client";

const STATE_COLORS: Record<string, string> = {
  IDLE: "bg-gray-500",
  CONNECTING: "bg-blue-500",
  AUTHENTICATED: "bg-blue-400",
  CONNECTED: "bg-green-500",
  DEGRADED: "bg-yellow-500",
  SAFE_MODE: "bg-red-600",
  RECOVERING: "bg-orange-500",
};

function SystemStateBadge({ state }: { state: string }) {
  const color = STATE_COLORS[state] ?? "bg-gray-600";
  return (
    <span className={`px-2 py-1 rounded text-white text-sm font-mono ${color}`}>
      {state}
    </span>
  );
}

const OPERATOR_ROLE_LABEL: Record<string, string> = {
  NO_OPERATOR: "",
  OPERATOR_ASSIGNED: "Assigned",
  ACTIVE_OPERATOR: "Active Operator",
  HANDOVER_PENDING: "Handover…",
  RECOVERING_OPERATOR: "Recovering",
};

export default function App() {
  const state = useSystemState();
  const session = useSession();
  const telemetry = useTelemetry(session.vehicleId);
  const vehicleAck = useVehicleAck(session.vehicleId);
  const [showSender, setShowSender] = useState(false)
  const [videoLatency, setVideoLatency] = useState<number | null>(null);
  const [showUserMgmt, setShowUserMgmt] = useState(false);

  const isConnected =
    state.system === "CONNECTED" || state.system === "DEGRADED";
  const isSafeMode = state.system === "SAFE_MODE";
  const isUnreachable = state.unreachable;

  const isAdmin = session.token ? parseTokenRole(session.token) === 'ADMIN' : false;

  // Show login overlay when no token is present
  if (!session.token) {
    return <LoginPanel onLogin={session.connect} />
  }

  return (
    <div className="min-h-screen bg-gray-900 text-white flex flex-col">
      {/* SAFE MODE overlay — blocks everything when system is in SAFE_MODE */}
      {isSafeMode && <SafeModeOverlay onResume={session.resume} />}

      {/* User management overlay — ADMIN only */}
      {showUserMgmt && session.token && (
        <UserManagementPanel
          token={session.token}
          currentUserId={session.operatorId ?? ''}
          onClose={() => setShowUserMgmt(false)}
        />
      )}

      {/* Header */}
      <header className="bg-gray-800 border-b border-gray-700 px-6 py-3 flex items-center justify-between">
        <h1 className="text-lg font-bold tracking-wide">
          AVOC — Teleoperation Control Center
        </h1>
        <div className="flex items-center gap-3">
          {/* Operator role badge */}
          {state.operator !== "NO_OPERATOR" && (
            <span className="text-xs font-mono text-gray-400 bg-gray-700 px-2 py-1 rounded">
              {OPERATOR_ROLE_LABEL[state.operator] ?? state.operator}
            </span>
          )}
          {isAdmin && (
            <button
              onClick={() => setShowUserMgmt(true)}
              className="px-3 py-1 rounded text-xs font-semibold bg-gray-700 hover:bg-gray-600 text-gray-300"
            >
              Benutzerverwaltung
            </button>
          )}
          <button
            onClick={() => setShowSender(v => !v)}
            className={`px-3 py-1 rounded text-xs font-semibold transition-colors ${
              showSender
                ? "bg-indigo-700 text-white"
                : "bg-gray-700 hover:bg-gray-600 text-gray-300"
            }`}
          >
            ⏺ Senden
          </button>
          <button
            onClick={session.disconnect}
            className="px-3 py-1 rounded text-xs font-semibold bg-gray-700 hover:bg-gray-600 text-gray-300"
          >
            Abmelden
          </button>
          <SystemStateBadge state={state.system} />
        </div>
      </header>

      {/* UNREACHABLE banner — backend nicht erreichbar */}
      {isUnreachable && (
        <div className="bg-red-950 border-b border-red-700 px-6 py-2 text-red-300 text-sm text-center font-semibold">
          ✕ Backend nicht erreichbar — Steuerung blockiert. Verbindung wird wiederhergestellt…
        </div>
      )}

      {/* DEGRADED banner */}
      {state.system === "DEGRADED" && (
        <div className="bg-yellow-900/50 border-b border-yellow-700 px-6 py-2 text-yellow-300 text-sm text-center">
          ⚠ DEGRADED — Video oder Telemetrie ausgefallen. Steuerung weiterhin
          möglich.
        </div>
      )}

      {/* Main Grid */}
      <main className="flex-1 grid grid-cols-3 grid-rows-2 gap-4 p-4 min-h-0">
        {/* Video Panel — 2 columns, 2 rows */}
        <VideoPanel
          sessionId={session.sessionId}
          vehicleId={session.vehicleId ?? ''}
          token={session.token}
          enabled={isConnected}
          onVideoLatency={setVideoLatency}
        />

        {/* Safety Panel */}
        <SafetyPanel
          systemState={state.system}
          sessionId={session.sessionId}
          vehicleId={session.vehicleId}
          wsClient={session.wsClient}
          token={session.token}
        />

        {/* Connection + Telemetry + Vehicle Selector */}
        <ConnectionPanel
          systemState={state.system}
          operatorState={state.operator}
          sessionId={session.sessionId}
          vehicleId={session.vehicleId}
          latency={session.latency}
          videoLatency={videoLatency}
          telemetry={telemetry}
          onStartSession={session.startSession}
          onEndSession={session.endSession}
        />
      </main>

      {/* Stream Sender — collapsible, toggle via header button */}
      {showSender && <StreamSenderPanel />}

      {/* Footer: Operator Inputs + Vehicle Feedback */}
      <footer className="bg-gray-800 border-t border-gray-700">
        <ControlPanel
          wsClient={session.wsClient}
          sessionId={session.sessionId}
          vehicleId={session.vehicleId}
          enabled={isConnected && !isUnreachable}
        />
        <InputIndicatorPanel telemetry={telemetry} ack={vehicleAck} />
      </footer>
    </div>
  );
}
