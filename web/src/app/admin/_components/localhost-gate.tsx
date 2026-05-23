"use client";

import { useState, useEffect, type ReactNode } from "react";
import { ShieldAlert } from "lucide-react";
import { fetchAdminAdapters } from "@/lib/api";

// @MX:NOTE: [AUTO] Localhost gate - blocks admin UI for non-localhost requests
// @MX:SPEC: SPEC-UI-002 Phase D1, REQ-LH-003

interface LocalhostGateProps {
  children: ReactNode;
}

type GateState = "loading" | "allowed" | "blocked";

const LOCALHOST_ADVISORY =
  "Admin UI is only accessible from localhost. Open this page on the machine running usearch-api.";

export function LocalhostGate({ children }: LocalhostGateProps) {
  const [state, setState] = useState<GateState>("loading");

  useEffect(() => {
    let cancelled = false;

    async function check() {
      try {
        await fetchAdminAdapters();
        if (!cancelled) setState("allowed");
      } catch {
        if (!cancelled) setState("blocked");
      }
    }

    check();
    return () => {
      cancelled = true;
    };
  }, []);

  if (state === "loading") {
    return (
      <div className="flex items-center justify-center min-h-[50vh]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  if (state === "blocked") {
    return (
      <div className="flex flex-col items-center justify-center min-h-[50vh] px-6 text-center">
        <ShieldAlert className="h-12 w-12 text-muted-foreground mb-4" />
        <p className="text-lg font-medium">{LOCALHOST_ADVISORY}</p>
      </div>
    );
  }

  return <>{children}</>;
}
