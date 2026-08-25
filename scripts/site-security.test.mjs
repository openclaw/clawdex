import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const html = fs.readFileSync(new URL("../index.html", import.meta.url), "utf8");
const expected = new Map([
  ["https://cdn.jsdelivr.net/npm/highlight.js@11.10.0/styles/atom-one-light.min.css", "sha384-w6Ujm1VWa9HYFqGc89oAPn/DWDi2gUamjNrq9DRvEYm2X3ClItg9Y9xs1ViVo5b5"],
  ["https://cdn.jsdelivr.net/npm/highlight.js@11.10.0/styles/atom-one-dark.min.css", "sha384-oaMLBGEzBOJx3UHwac0cVndtX5fxGQIfnAeFZ35RTgqPcYlbprH9o9PUV/F8Le07"],
  ["https://cdn.jsdelivr.net/npm/marked@14.1.3/marked.min.js", "sha384-k8o8HikHweyzW55Wd3wl18ovJj6vHVYNQeQbeSM0fxx+0WiH4TcccOG9uz8Xd2JR"],
  ["https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.10.0/highlight.min.js", "sha384-GdEWAbCjn+ghjX0gLx7/N1hyTVmPAjdC2OvoAA0RyNcAOhqwtT8qnbCxWle2+uJX"],
  ["https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.10.0/languages/go.min.js", "sha384-Mtb4EH3R9NMDME1sPQALOYR8KGqwrXAtmc6XGxDd0XaXB23irPKsuET0JjZt5utI"],
  ["https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.10.0/languages/dockerfile.min.js", "sha384-jg4vR4ePpACdBVLAe+31BrI3MW4sfv1AS62HlXRXmQWk2q98yJqKR5VxHzuABw8X"],
  ["https://cdn.jsdelivr.net/npm/dompurify@3.2.4/dist/purify.min.js", "sha384-eEu5CTj3qGvu9PdJuS+YlkNi7d2XxQROAFYOr59zgObtlcux1ae1Il3u7jvdCSWu"],
]);

function attribute(tag, name) {
  return tag.match(new RegExp(`\\b${name}="([^"]+)"`))?.[1] || "";
}

const externalAssets = [
  ...html.matchAll(/<script\b[^>]*\bsrc="https:\/\/[^"]+"[^>]*>/g),
  ...html.matchAll(/<link\b(?=[^>]*\brel="stylesheet")(?=[^>]*\bhref="https:\/\/)[^>]*>/g),
].map((match) => match[0]);

test("pins the exact external asset set", () => {
  assert.equal(externalAssets.length, expected.size);
  const actual = new Map(
    externalAssets.map((tag) => [
      attribute(tag, tag.startsWith("<script") ? "src" : "href"),
      attribute(tag, "integrity"),
    ]),
  );
  assert.deepEqual(actual, expected);
});

test("has no unhashed external scripts or stylesheets", () => {
  for (const tag of externalAssets) {
    assert.match(attribute(tag, "integrity"), /^sha384-[A-Za-z0-9+/]+={0,2}$/);
    assert.equal(attribute(tag, "crossorigin"), "anonymous");
  }
});

test("fails closed when renderer dependencies are unavailable", () => {
  assert.match(html, /const missing = \["marked", "hljs", "DOMPurify"\]/);
  assert.match(html, /paragraph\.textContent = message/);
  assert.doesNotMatch(html, /window\.DOMPurify\s*\?/);
  assert.match(html, /DOMPurify\.sanitize\(marked\.parse\(md\)/);
  assert.equal([...html.matchAll(/article\.innerHTML/g)].length, 1);
});
