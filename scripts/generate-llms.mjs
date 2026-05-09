#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const cname = fs.readFileSync(path.join(repoRoot, "CNAME"), "utf8").trim();
const origin = "https://" + cname;
const html = fs.readFileSync(path.join(repoRoot, "index.html"), "utf8");
const title = text(html.match(/<title[^>]*>([\s\S]*?)<\/title>/i)?.[1]) || "clawdex";
const description = attr(html.match(/<meta\s+name=["']description["']\s+content=["']([^"']*)["'][^>]*>/i)?.[1] || "Local-first contact index.");
const lines = [
  "# clawdex",
  "",
  description,
  "",
  "Canonical pages:",
  "- " + title + ": " + origin + "/",
  "",
  "Source: https://github.com/openclaw/clawdex",
  "",
  "Guidance for agents:",
  "- Use this file as a site index, not a full-site corpus.",
  "",
];
fs.writeFileSync(path.join(repoRoot, "llms.txt"), lines.join("\n"), "utf8");
console.log("wrote llms.txt");

function text(value) {
  return attr(value || "").replace(/<[^>]+>/g, "").replace(/\s+/g, " ").trim();
}
function attr(value) {
  return String(value || "").replace(/&mdash;/g, "-").replace(/&amp;/g, "&").replace(/&nbsp;/g, " ").trim();
}
