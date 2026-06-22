"use client";

/**
 * CapabilitiesTable — renders the auto-extracted Capabilities fields from a
 * per-adapter JSON file plus a footer showing the source path + line number.
 *
 * HARD: no per-page hand-overridden field values are accepted.
 * To change a value, modify the underlying adapter Go source (triggers drift gate).
 *
 * SPEC-DOC-002 REQ-ADPDOC-008
 */

interface CapabilitiesData {
  sourceID: string;
  requiresAuth: boolean;
  authEnvVars: string[];
  rateLimitPerMin: number;
  defaultMaxResults: number;
  sourcePath: string;
  sourceLine: number;
  extractedAt: string;
}

interface Props {
  /** Path to the capabilities JSON relative to the component file, e.g.
   *  `../_generated/reddit.capabilities.json`. Passed as imported JSON object. */
  data: CapabilitiesData;
}

/**
 * CapabilitiesTable renders the 5 extracted fields and source footer.
 * MDX pages import the JSON directly and pass it as `data`.
 */
export function CapabilitiesTable({ data }: Props) {
  return (
    <div className="my-4">
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr className="border-b">
            <th className="text-left py-1 pr-4 font-semibold w-1/3">Field</th>
            <th className="text-left py-1 font-semibold">Value</th>
          </tr>
        </thead>
        <tbody>
          <tr className="border-b">
            <td className="py-1 pr-4 font-mono text-xs">SourceID</td>
            <td className="py-1 font-mono">{data.sourceID}</td>
          </tr>
          <tr className="border-b">
            <td className="py-1 pr-4 font-mono text-xs">RequiresAuth</td>
            <td className="py-1">{data.requiresAuth ? "Yes" : "No"}</td>
          </tr>
          <tr className="border-b">
            <td className="py-1 pr-4 font-mono text-xs">AuthEnvVars</td>
            <td className="py-1">
              {data.authEnvVars.length > 0 ? (
                <ul className="list-none p-0 m-0 space-y-0.5">
                  {data.authEnvVars.map((v) => (
                    <li key={v}>
                      <code>{v}</code>
                    </li>
                  ))}
                </ul>
              ) : (
                <span className="text-gray-500">—</span>
              )}
            </td>
          </tr>
          <tr className="border-b">
            <td className="py-1 pr-4 font-mono text-xs">RateLimitPerMin</td>
            <td className="py-1">
              {data.rateLimitPerMin === 0
                ? "0 (see rate limits section)"
                : data.rateLimitPerMin}
            </td>
          </tr>
          <tr>
            <td className="py-1 pr-4 font-mono text-xs">DefaultMaxResults</td>
            <td className="py-1">{data.defaultMaxResults}</td>
          </tr>
        </tbody>
      </table>
      <p className="mt-2 text-xs text-gray-500">
        Extracted from{" "}
        <code>
          {data.sourcePath}:{data.sourceLine}
        </code>{" "}
        — auto-generated, do not edit by hand.
      </p>
    </div>
  );
}

export default CapabilitiesTable;
