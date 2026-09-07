import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const script = fileURLToPath(new URL("./check-release.mjs", import.meta.url));
const notes = "## 0.2.0 - 2026-09-05\n\n- **Highlights:** Local contact imports.\n- Keep Unicode: é.\n";

function fixture(t, changelog = notes) {
  const dir = mkdtempSync(join(tmpdir(), "clawdex-release-check-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  writeFileSync(join(dir, "CHANGELOG.md"), changelog);
  return dir;
}

function run(dir, ...args) {
  return spawnSync(process.execPath, [script, ...args], { cwd: dir, encoding: "utf8" });
}

test("release preflight prints only the requested dated section in order", (t) => {
  const dir = fixture(t, "# Changelog\n\n## Unreleased\n\n- Future work.\n\n" + notes + "\n## 0.1.0 - 2026-05-08\n\n- Old.\n");
  for (const version of ["0.2.0", "v0.2.0"]) {
    const result = run(dir, version);
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, notes);
  }
});

test("release preflight rejects malformed versions and arguments", (t) => {
  const dir = fixture(t);
  for (const args of [[], ["0.2"], ["00.2.0"], ["0.2.0-rc.1"], ["0.2.0", "extra"], ["0.2.0\nmalicious"]]) {
    const result = run(dir, ...args);
    assert.equal(result.status, 1);
    assert.equal(result.stdout, "");
  }
});

test("release preflight rejects missing, duplicate, undated, invalid-date and empty notes", (t) => {
  for (const changelog of [
    "## Unreleased\n\n- Pending.\n",
    notes + "\n" + notes,
    notes.replace("2026-09-05", "Unreleased"),
    notes.replace("2026-09-05", "2026-02-31"),
    "## 0.2.0 - 2026-09-05\n\n## 0.1.0 - 2026-05-08\n\n- Old.\n",
  ]) {
    const result = run(fixture(t, changelog), "0.2.0");
    assert.equal(result.status, 1, changelog);
    assert.equal(result.stdout, "");
  }
});

test("release preflight ignores headings inside code fences and comments", (t) => {
  const decoys = "```md\n" + notes + "```\n<!--\n" + notes + "-->\n";
  const result = run(fixture(t, decoys + notes), "0.2.0");
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, notes);
});

test("release preflight enforces every present version file", (t) => {
  for (const name of [".release-version", "VERSION", "version.txt", "package.json"]) {
    const dir = fixture(t);
    const contents = (version) => name === "package.json" ? JSON.stringify({ version }) : version + "\n";
    writeFileSync(join(dir, name), contents("v0.2.0"));
    assert.equal(run(dir, "0.2.0").status, 0, name);
    writeFileSync(join(dir, name), contents("0.1.0"));
    const mismatch = run(dir, "0.2.0");
    assert.equal(mismatch.status, 1, name);
    assert.match(mismatch.stderr, /version does not match/);
    assert.equal(mismatch.stdout, "");
  }
  const dir = fixture(t);
  writeFileSync(join(dir, "package.json"), JSON.stringify({ private: true }));
  assert.equal(run(dir, "0.2.0").status, 0);
});
