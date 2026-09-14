/**
 * User-curated filesystem entry points for a daemon machine.
 *
 * These live in workspace.settings so the browser and the daemon agree about
 * which paths are intended project locations. A location is a shortcut, not a
 * sandbox: a system location may intentionally point at `/` for infrastructure
 * agents. The daemon-side browser still validates every traversal request.
 */
export interface ProjectLocation {
  id: string;
  daemon_id: string;
  label: string;
  path: string;
  system?: boolean;
}

export const PROJECT_LOCATIONS_SETTINGS_KEY = "project_locations_v1";

function isLocation(value: unknown): value is ProjectLocation {
  if (!value || typeof value !== "object") return false;
  const entry = value as Record<string, unknown>;
  return (
    typeof entry.id === "string" &&
    typeof entry.daemon_id === "string" &&
    typeof entry.label === "string" &&
    typeof entry.path === "string" &&
    entry.path.startsWith("/")
  );
}

/** Parse defensively: settings JSON is user data and older servers know none of it. */
export function projectLocationsFromSettings(settings: Record<string, unknown> | null | undefined): ProjectLocation[] {
  const raw = settings?.[PROJECT_LOCATIONS_SETTINGS_KEY];
  if (!Array.isArray(raw)) return [];
  return raw
    .filter(isLocation)
    .map((entry) => ({
      ...entry,
      label: entry.label.trim(),
      path: entry.path.replace(/\/+$/, "") || "/",
      system: entry.system === true,
    }))
    .filter((entry) => entry.label.length > 0 && entry.path.startsWith("/"));
}

export function projectLocationsSettings(
  current: Record<string, unknown> | null | undefined,
  locations: ProjectLocation[],
): Record<string, unknown> {
  return { ...(current ?? {}), [PROJECT_LOCATIONS_SETTINGS_KEY]: locations };
}
