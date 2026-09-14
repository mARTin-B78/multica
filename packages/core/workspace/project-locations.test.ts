import { describe, expect, it } from "vitest";
import {
  PROJECT_LOCATIONS_SETTINGS_KEY,
  projectLocationsFromSettings,
  projectLocationsSettings,
} from "./project-locations";

describe("project locations workspace setting", () => {
  it("keeps only well-formed absolute locations and normalizes trailing slashes", () => {
    expect(
      projectLocationsFromSettings({
        [PROJECT_LOCATIONS_SETTINGS_KEY]: [
          { id: "drive", daemon_id: "dgx", label: "Drive", path: "/mnt/drive/" },
          { id: "bad", daemon_id: "dgx", label: "Relative", path: "projects" },
          { id: "system", daemon_id: "nuc", label: "System", path: "/", system: true },
        ],
      }),
    ).toEqual([
      { id: "drive", daemon_id: "dgx", label: "Drive", path: "/mnt/drive", system: false },
      { id: "system", daemon_id: "nuc", label: "System", path: "/", system: true },
    ]);
  });

  it("updates locations without dropping unrelated workspace settings", () => {
    expect(
      projectLocationsSettings(
        { github_enabled: true },
        [{ id: "docker", daemon_id: "dgx", label: "Docker", path: "/home/sparky/Docker" }],
      ),
    ).toEqual({
      github_enabled: true,
      [PROJECT_LOCATIONS_SETTINGS_KEY]: [
        { id: "docker", daemon_id: "dgx", label: "Docker", path: "/home/sparky/Docker" },
      ],
    });
  });
});
