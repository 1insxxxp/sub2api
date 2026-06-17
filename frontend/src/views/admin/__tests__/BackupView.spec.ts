import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const currentDir = dirname(fileURLToPath(import.meta.url));
const backupViewSource = readFileSync(
  resolve(currentDir, "../BackupView.vue"),
  "utf8",
);

describe("admin BackupView layout", () => {
  it("uses shared admin surfaces for backup settings and operations", () => {
    expect(backupViewSource).toContain('data-test="backup-s3-surface"');
    expect(backupViewSource).toContain('data-test="backup-schedule-surface"');
    expect(backupViewSource).toContain('data-test="backup-operations-surface"');
    expect(backupViewSource).toContain("admin-panel-header");
    expect(backupViewSource).toContain("admin-toolbar-surface");
    expect(backupViewSource).not.toContain('class="card p-6"');
  });
});
