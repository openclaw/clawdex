import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
import test from "node:test";

const html = fs.readFileSync(new URL("../index.html", import.meta.url), "utf8");
const script = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].at(-1)[1];

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

function viewer() {
  const requests = [];
  const headings = [];
  const anchors = [];
  const article = {
    innerHTML: "",
    message: "",
    replaceChildren(node) { this.message = node.textContent; this.innerHTML = ""; },
    querySelectorAll(selector) { return selector === "[id]" ? [...headings, ...anchors].filter(element => element.id) : headings; },
  };
  const elements = {
    article,
    card: { dataset: {} },
    "meta-slug": {},
    "meta-link": {},
  };
  const location = { hash: "#/index" };
  let route;
  const window = {
    marked: { use() {}, parse(value) { return value; } },
    hljs: { getLanguage() { return true; } },
    DOMPurify: { sanitize(value) { return value; } },
    scrollTo() {},
    matchMedia() { return { matches: false }; },
    addEventListener(name, handler) { if (name === "hashchange") route = handler; },
  };
  const document = {
    getElementById(id) { return elements[id] || headings.find(h => h.id === id); },
    createElement() { return {}; },
    querySelectorAll() { return []; },
    querySelector() { return { querySelector() { return { addEventListener() {} }; } }; },
  };
  vm.runInNewContext(script, {
    window, document, location, ...window,
    fetch(url) { const request = deferred(); requests.push({ url, ...request }); return request.promise; },
  });
  return {
    article, elements, requests, headings, anchors,
    navigate(hash) { location.hash = hash; route(); },
  };
}

const settle = () => new Promise(resolve => setImmediate(resolve));
const response = text => ({ ok: true, text: async () => text });

test("late page success cannot overwrite the current route", async () => {
  const v = viewer();
  v.navigate("#/notes");
  v.requests[1].resolve(response("Notes content"));
  await settle();
  v.requests[0].resolve(response("Old overview"));
  await settle();
  assert.equal(v.article.innerHTML, "Notes content");
  assert.equal(v.elements["meta-slug"].textContent, "docs/notes.md");
});

test("late page failure cannot replace a successful current route", async () => {
  const v = viewer();
  v.navigate("#/people");
  v.requests[1].resolve(response("People content"));
  await settle();
  v.requests[0].reject(new Error("old request failed"));
  await settle();
  assert.equal(v.article.innerHTML, "People content");
});

test("each navigation owns its render even when revisiting the same slug", async () => {
  const v = viewer();
  v.navigate("#/notes");
  v.navigate("#/index");
  v.requests[2].resolve(response("Current overview"));
  await settle();
  v.requests[0].resolve(response("Old overview"));
  v.requests[1].resolve(response("Old notes"));
  await settle();
  assert.equal(v.article.innerHTML, "Current overview");
});

test("rendered headings receive unique fragment IDs and can be targeted", async () => {
  const v = viewer();
  let scrolled = false;
  v.headings.push(
    { textContent: "Sync (preview-only)", scrollIntoView() { scrolled = true; } },
    { textContent: "Sync (preview-only)", scrollIntoView() {} },
  );
  v.navigate("#/imports#sync-preview-only");
  v.requests[1].resolve(response("Imports content"));
  await settle();
  assert.deepEqual(v.headings.map(h => h.id), ["sync-preview-only", "sync-preview-only-1"]);
  assert.equal(scrolled, true);
  v.requests[0].resolve(response("Old overview"));
  await settle();
});

test("fragment IDs avoid collisions with numbered headings and decode Unicode", async () => {
  const v = viewer();
  let scrolled = false;
  for (const text of ["A", "A", "A-1", "Über"]) {
    v.headings.push({ textContent: text, scrollIntoView() { scrolled = text === "Über"; } });
  }
  v.navigate("#/notes#%C3%BCber");
  v.requests[1].resolve(response("Notes content"));
  await settle();
  assert.deepEqual(v.headings.map(h => h.id), ["a", "a-1", "a-1-1", "über"]);
  assert.equal(scrolled, true);
  v.requests[0].resolve(response("Old overview"));
  await settle();
});


test("generated headings preserve explicit anchors throughout the article", async () => {
  const v = viewer();
  let target;
  v.headings.push(
    { textContent: "Setup", scrollIntoView() { target = "generated"; } },
    { textContent: "Advanced", id: "setup", scrollIntoView() { target = "explicit"; } },
    { textContent: "Overview", scrollIntoView() {} },
  );
  v.anchors.push({ id: "overview" });
  v.navigate("#/notes#setup");
  v.requests[1].resolve(response("Notes content"));
  await settle();
  assert.deepEqual(v.headings.map(h => h.id), ["setup-1", "setup", "overview-1"]);
  assert.equal(target, "explicit");
  v.requests[0].resolve(response("Old overview"));
  await settle();
});
