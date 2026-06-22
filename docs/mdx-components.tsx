import { useMDXComponents as useNextraComponents } from "nextra-theme-docs";
import { StatusBadge } from "./components/StatusBadge";
import { CapabilitiesTable } from "./components/CapabilitiesTable";
import { AdapterCatalog } from "./components/AdapterCatalog";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useMDXComponents(components?: any): any {
  return {
    ...useNextraComponents(components),
    StatusBadge,
    CapabilitiesTable,
    AdapterCatalog,
  };
}
