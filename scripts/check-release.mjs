#!/usr/bin/env node
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export function checkRelease(versionArg, root = process.cwd()) {
  const match = /^v?((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))$/.exec(versionArg ?? "");
  if (!match) throw new Error("version must be stable SemVer, for example 0.2.0");
  const version = match[1];
  const read = (name) => readFileSync(resolve(root, name), "utf8");
  const lines = read("CHANGELOG.md").split(/\r?\n/);
  const headings = [];
  let fence;
  let comment = false;
  for (const [index, line] of lines.entries()) {
    if (fence) {
      if (new RegExp(`^ {0,3}${fence[0]}{${fence.length},}\\s*$`).test(line)) fence = undefined;
      continue;
    }
    if (!comment) {
      const opening = /^ {0,3}(`{3,}|~{3,})/.exec(line);
      if (opening) {
        fence = opening[1];
        continue;
      }
      if (/^##\s/.test(line)) headings.push({ index, line });
    }
    let cursor = 0;
    while (cursor < line.length) {
      const marker = comment ? "-->" : "<!--";
      const at = line.indexOf(marker, cursor);
      if (at < 0) break;
      comment = !comment;
      cursor = at + marker.length;
    }
  }
  const escaped = version.replaceAll(".", "\\.");
  const candidates = headings.filter(({ line }) => new RegExp(`^## v?${escaped}(?:\\s|$)`).test(line));
  if (candidates.length !== 1) throw new Error(`CHANGELOG.md needs exactly one section for ${version}`);
  const heading = candidates[0];
  const date = new RegExp(`^## v?${escaped} - (\\d{4}-\\d{2}-\\d{2})$`).exec(heading.line)?.[1];
  if (!date || Number.isNaN(Date.parse(date)) || new Date(date).toISOString().slice(0, 10) !== date) {
    throw new Error(`CHANGELOG.md needs a dated ${version} heading (YYYY-MM-DD)`);
  }
  const end = headings.find(({ index }) => index > heading.index)?.index ?? lines.length;
  const body = lines.slice(heading.index + 1, end);
  if (!body.some((line) => line.startsWith("- "))) throw new Error(`release notes for ${version} are empty`);

  for (const name of [".release-version", "VERSION", "version.txt", "package.json"]) {
    if (!existsSync(resolve(root, name))) continue;
    const actual = name === "package.json" ? JSON.parse(read(name)).version : read(name).trim();
    if (name === "package.json" && actual === undefined) continue;
    if (typeof actual !== "string" || actual.replace(/^v/, "") !== version) {
      throw new Error(`${name} version does not match ${version}`);
    }
  }
  return `${lines.slice(heading.index, end).join("\n").trimEnd()}\n`;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    if (process.argv.length !== 3) throw new Error("usage: node scripts/check-release.mjs VERSION");
    process.stdout.write(checkRelease(process.argv[2]));
  } catch (error) {
    process.stderr.write(`release check: ${error.message}\n`);
    process.exitCode = 1;
  }
}
