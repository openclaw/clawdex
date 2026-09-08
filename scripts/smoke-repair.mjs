import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const binary = resolve(process.argv[2]);
const scratch = mkdtempSync(join(tmpdir(), "clawdex-repair-"));
const config = join(scratch, "config.toml");
const repo = join(scratch, "contacts");
function run(...args) {
  const result = spawnSync(binary, ["--config", config, "--repo", repo, "--json", ...args], {
    encoding: "utf8",
    timeout: 30_000,
  });
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
  return JSON.parse(result.stdout);
}

try {
  run("init", repo, "--remote", "");
  const personDir = join(repo, "people", "ada");
  mkdirSync(personDir, { recursive: true });
  const person = join(personDir, "person.md");
  const original = "---\nid: person_1\nname: Ada Lovelace\ntags: [math\n---\n# Ada\n";
  writeFileSync(person, original);
  assert.equal(run("doctor", "--repair").repaired, 1);
  const repairs = join(repo, ".clawdex", "repairs");
  const backups = readdirSync(repairs, { recursive: true, withFileTypes: true })
    .filter((entry) => entry.isFile());
  assert.equal(backups.length, 1);
  assert.equal(readFileSync(join(backups[0].parentPath, backups[0].name), "utf8"), original);
  assert.notEqual(readFileSync(person, "utf8"), original);
  assert.equal(run("doctor", "--repair").repaired, 0);
  assert.equal(run("person", "show", "person_1").name, "Ada Lovelace");
  console.log("PASS: doctor repaired the contact, preserved the original backup, and is idempotent.");
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
