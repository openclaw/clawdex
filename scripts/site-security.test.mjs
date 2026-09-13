import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const html = fs.readFileSync(new URL("../index.html", import.meta.url), "utf8");
const viewer = fs.readFileSync(new URL("./site.js", import.meta.url), "utf8");
const expected = new Map([
  ["https://cdn.jsdelivr.net/npm/highlight.js@11.12.0/styles/atom-one-light.min.css", "sha384-w6Ujm1VWa9HYFqGc89oAPn/DWDi2gUamjNrq9DRvEYm2X3ClItg9Y9xs1ViVo5b5"],
  ["https://cdn.jsdelivr.net/npm/highlight.js@11.12.0/styles/atom-one-dark.min.css", "sha384-oaMLBGEzBOJx3UHwac0cVndtX5fxGQIfnAeFZ35RTgqPcYlbprH9o9PUV/F8Le07"],
  ["https://cdn.jsdelivr.net/npm/marked@18.0.13/lib/marked.umd.js", "sha384-Jy8qDMspJASzATgFngF2ompIKy0StbCcvuTE65mxDm/E0/YSIF6Ndc+5V7bbwRcw"],
  ["https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.12.0/highlight.min.js", "sha384-wjfDDhOPPdjtva8vWBhWeVprSpmxisEu5aYT3q1JyACqXpdKpo3PWZTMVq24MBix"],
  ["https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.12.0/languages/go.min.js", "sha384-orYKHAs3chK3oDMQLy5ywrzoY8z9zvzfmNIjmVxKXioAUtwDhP+xf6THWYSI/43Y"],
  ["https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.12.0/languages/dockerfile.min.js", "sha384-/zu1pI8+9j/v/qNlCRRyidiBhGdxfvGwOLXEPBXpKc77eFNUAhccr0WglEQ+x9La"],
  ["https://cdn.jsdelivr.net/npm/dompurify@3.4.15/dist/purify.min.js", "sha384-uUMu9JDY09vBzRf9SPcK2VgUj+W/70J6Soc+Dded5P474ElQ63iv9j5N3DE7Kp3N"],
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
  assert.ok(html.includes('<script src="scripts/site.js"></script>'));
  assert.match(viewer, /const missing = \["marked", "hljs", "DOMPurify"\]/);
  assert.match(viewer, /paragraph\.textContent = message/);
  assert.doesNotMatch(viewer, /window\.DOMPurify\s*\?/);
  assert.match(viewer, /DOMPurify\.sanitize\(marked\.parse\(md\)/);
  assert.equal([...viewer.matchAll(/article\.innerHTML/g)].length, 1);
});
