import type { Metadata } from "next";
import { LocalhostGate } from "./_components/localhost-gate";
import { AdapterStatusPanel } from "./_components/adapter-status-panel";
import { AuditViewer } from "./_components/audit-viewer";

export const metadata: Metadata = {
  title: "Admin — Universal Search",
  description: "Admin panel for adapters and query audit",
};

export default function AdminPage() {
  return (
    <div className="flex flex-col min-h-full">
      <header className="border-b border-border px-6 py-4 md:px-12">
        <h1 className="text-xl font-semibold pl-10 md:pl-0">Admin</h1>
        <p className="text-sm text-muted-foreground mt-1 pl-10 md:pl-0">
          Adapter status, API keys, and query audit
        </p>
      </header>

      <LocalhostGate>
        <div className="flex-1 px-6 py-6 md:px-12 space-y-8">
          <AdapterStatusPanel />
          <AuditViewer />
        </div>
      </LocalhostGate>
    </div>
  );
}
