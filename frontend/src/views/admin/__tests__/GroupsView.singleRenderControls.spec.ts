import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const currentDir = dirname(fileURLToPath(import.meta.url));
const groupsViewSource = readFileSync(
  resolve(currentDir, "../GroupsView.vue"),
  "utf8",
);
const sharedConfigSectionsSource = readFileSync(
  resolve(currentDir, "../GroupSharedConfigSections.vue"),
  "utf8",
);

const countOccurrences = (source: string, needle: string) =>
  source.split(needle).length - 1;

describe("groups form single-render controls", () => {
  it("mounts the shared configuration sections once per create/edit form", () => {
    expect(
      countOccurrences(groupsViewSource, "<GroupSharedConfigSections"),
    ).toBe(2);
  });

  it("keeps each shared configuration section in one template only", () => {
    expect(
      countOccurrences(
        sharedConfigSectionsSource,
        'imagePricingI18nKey(form.platform, "title")',
      ),
    ).toBe(1);
    expect(
      countOccurrences(
        sharedConfigSectionsSource,
        "admin.groups.videoPricing.title",
      ),
    ).toBe(1);
    expect(
      countOccurrences(sharedConfigSectionsSource, "admin.groups.peakRate.enable"),
    ).toBe(1);
    expect(
      countOccurrences(
        sharedConfigSectionsSource,
        "admin.groups.supportedScopes.title",
      ),
    ).toBe(1);
    expect(
      countOccurrences(sharedConfigSectionsSource, "admin.groups.mcpXml.title"),
    ).toBe(1);
    expect(
      countOccurrences(sharedConfigSectionsSource, "admin.groups.claudeCode.title"),
    ).toBe(1);
    expect(
      countOccurrences(groupsViewSource, "admin.groups.imagePricing.title"),
    ).toBe(0);
    expect(
      countOccurrences(groupsViewSource, "admin.groups.videoPricing.title"),
    ).toBe(0);
    expect(countOccurrences(groupsViewSource, "admin.groups.peakRate.enable")).toBe(
      0,
    );
    expect(
      countOccurrences(groupsViewSource, "admin.groups.supportedScopes.title"),
    ).toBe(0);
    expect(countOccurrences(groupsViewSource, "admin.groups.mcpXml.title")).toBe(
      0,
    );
    expect(countOccurrences(groupsViewSource, "admin.groups.claudeCode.title")).toBe(
      0,
    );
  });

  it("keeps upstream cross-platform account copying for composite groups", () => {
    expect(groupsViewSource).toContain(
      'targetPlatform === "composite" || sourcePlatform === targetPlatform',
    );
    expect(
      countOccurrences(groupsViewSource, "canCopyAccountsFromGroup("),
    ).toBe(2);
  });
});
