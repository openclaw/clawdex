import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const generator = fs.readFileSync(new URL("./generate-llms.mjs", import.meta.url), "utf8");

function generate({ title, description }) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "clawdex-llms-"));
  fs.mkdirSync(path.join(root, "scripts"));
  fs.writeFileSync(path.join(root, "scripts/generate-llms.mjs"), generator);
  fs.writeFileSync(path.join(root, "CNAME"), "clawdex.sh\n");
  fs.writeFileSync(
    path.join(root, "index.html"),
    `<title>${title}</title><meta name="description" content="${description}">`,
  );
  const result = spawnSync(process.execPath, ["scripts/generate-llms.mjs"], {
    cwd: root,
    encoding: "utf8",
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, "wrote llms.txt\n");
  const output = fs.readFileSync(path.join(root, "llms.txt"), "utf8");
  fs.rmSync(root, { recursive: true, force: true });
  return output;
}

const fixtures = [
  ["current metadata", "Clawdex — local-first contact index", "Personal contact index backed by markdown and private Git.", "Clawdex — local-first contact index", "Personal contact index backed by markdown and private Git."],
  ["known entities", "Clawdex &mdash; contacts", "Local&nbsp;first &amp; private", "Clawdex - contacts", "Local first & private"],
  ["nested entities", "Clawdex &amp;mdash; contacts", "Local&amp;nbsp;first", "Clawdex &mdash; contacts", "Local&nbsp;first"],
  ["markup-like text", "Clawdex <contacts>", "Use <script and <notes> literally", "Clawdex <contacts>", "Use <script and <notes> literally"],
  ["whitespace", "  Clawdex \n\t contacts  ", "  Local \n\t first  ", "Clawdex contacts", "Local first"],
];

for (const [name, title, description, expectedTitle, expectedDescription] of fixtures) {
  test(`generate-llms: ${name}`, () => {
    const output = generate({ title, description });
    assert.match(output, new RegExp(`^${escapeRegex(expectedDescription)}$`, "m"));
    assert.match(output, new RegExp(`^- ${escapeRegex(expectedTitle)}: https://clawdex\\.sh/$`, "m"));
  });
}

function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
