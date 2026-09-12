(function () {
  const KNOWN = new Set([
    "index", "install", "quickstart",
    "people", "notes", "timeline", "search", "avatars",
    "imports", "vcard-export", "git-sync",
    "markdown-storage", "doctor", "config",
    "RELEASING"
  ]);
  const TAB_LABEL = {
    "index": "overview",
    "install": "install",
    "quickstart": "quickstart",
    "people": "people",
    "notes": "notes",
    "timeline": "timeline",
    "search": "search",
    "avatars": "avatars",
    "imports": "imports",
    "vcard-export": "vcard export",
    "git-sync": "git sync",
    "markdown-storage": "storage",
    "doctor": "doctor",
    "config": "config",
    "RELEASING": "releasing"
  };

  const article = document.getElementById("article");
  const card    = document.getElementById("card");
  const metaSlug = document.getElementById("meta-slug");
  const metaLink = document.getElementById("meta-link");
  const cache = new Map();
  let activeLoad = 0;

  function showMessage(message, error = false) {
    const paragraph = document.createElement("p");
    paragraph.className = "placeholder" + (error ? " error" : "");
    paragraph.textContent = message;
    article.replaceChildren(paragraph);
  }

  const missing = ["marked", "hljs", "DOMPurify"].filter(name => !window[name]);
  if (missing.length) {
    showMessage("documentation viewer unavailable: missing " + missing.join(", ") + ". Reload to retry.", true);
    return;
  }
  const { marked, hljs, DOMPurify } = window;

  // Register short aliases highlight.js doesn't ship by default.
  const ALIASES = { toml: "ini", sh: "bash", shell: "bash", zsh: "bash", txt: "plaintext", text: "plaintext" };
  function resolveLang(lang) {
    if (!lang) return "";
    if (hljs.getLanguage(lang)) return lang;
    const a = ALIASES[lang.toLowerCase()];
    if (a && hljs.getLanguage(a)) return a;
    return "";
  }

  const renderer = {
    link(token) {
      const href = token.href || "";
      const text = token.text || "";
      const title = token.title || "";
      let h = String(href);
      const m = h.match(/^([A-Za-z0-9_-]+)\.md(#.+)?$/);
      if (m) h = "#/" + m[1] + (m[2] || "");
      const t = title ? ` title="${title}"` : "";
      const ext = /^https?:/.test(h) ? ` target="_blank" rel="noopener"` : "";
      return `<a href="${h}"${t}${ext}>${text}</a>`;
    },
    code(token) {
      const c = token.text || "";
      const requested = (token.lang || "").trim();
      const lang = resolveLang(requested);
      let html;
      try {
        if (lang) {
          html = hljs.highlight(c, { language: lang, ignoreIllegals: true }).value;
        } else {
          const auto = hljs.highlightAuto(c, ["bash", "go", "json", "yaml", "ini", "markdown", "javascript", "xml"]);
          html = auto.value;
        }
      } catch (_) {
        html = c.replace(/[&<>]/g, s => ({"&":"&amp;","<":"&lt;",">":"&gt;"}[s]));
      }
      const cls = lang ? `language-${lang}` : (requested ? `language-${requested}` : "");
      return `<pre><code class="hljs ${cls}">${html}</code></pre>`;
    }
  };
  marked.use({ gfm: true, breaks: false, renderer });

  function setActive(slug) {
    document.querySelectorAll("aside.tabs a.tab").forEach(a => {
      a.classList.toggle("active", a.dataset.slug === slug);
    });
    card.dataset.tab = TAB_LABEL[slug] || slug;
    const file = (slug === "RELEASING") ? "RELEASING.md" : (slug + ".md");
    metaSlug.textContent = "docs/" + file;
    metaLink.href = `https://github.com/openclaw/clawdex/blob/main/docs/${file}`;
  }

  async function load(slug) {
    const request = ++activeLoad;
    setActive(slug);
    showMessage("flipping to " + slug + "…");
    let md = cache.get(slug);
    if (!md) {
      try {
        const file = (slug === "RELEASING") ? "RELEASING.md" : (slug + ".md");
        const res = await fetch("docs/" + file, { headers: { "Accept": "text/plain" } });
        if (!res.ok) throw new Error(res.status + " " + res.statusText);
        md = await res.text();
        if (request !== activeLoad) return;
        cache.set(slug, md);
      } catch (err) {
        if (request !== activeLoad) return;
        showMessage("card not found: " + slug + " — " + err.message + ". Reload to retry.", true);
        return;
      }
    }
    const clean = DOMPurify.sanitize(marked.parse(md), { ADD_ATTR: ['target'] });
    article.innerHTML = clean;
    const usedIds = new Set([...article.querySelectorAll("[id]")].map(element => element.id));
    for (const heading of article.querySelectorAll("h1, h2, h3, h4, h5, h6")) {
      if (heading.id) continue;
      const base = heading.textContent.trim().toLowerCase()
        .replace(/[^\p{L}\p{N}\s_-]/gu, "").replace(/\s/g, "-") || "section";
      let id = base;
      for (let suffix = 1; usedIds.has(id); suffix++) id = `${base}-${suffix}`;
      usedIds.add(id);
      heading.id = id;
    }

    const sub = fragmentId();
    if (sub) {
      const el = [...article.querySelectorAll("[id]")].find(element => element.id === sub);
      if (el) { el.scrollIntoView({ behavior: "smooth", block: "start" }); return; }
    }
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function fragmentId() {
    const fragment = location.hash.split("#")[2] || "";
    try { return decodeURIComponent(fragment); }
    catch { return fragment; }
  }

  function route() {
    const h = location.hash.replace(/^#\/?/, "").split("#")[0] || "index";
    const slug = KNOWN.has(h) ? h : "index";
    load(slug);
  }

  const tabsAside = document.querySelector("aside.tabs");
  const menuBtn = tabsAside.querySelector(".menu-button");
  menuBtn.addEventListener("click", () => {
    const open = tabsAside.classList.toggle("collapsed");
    menuBtn.setAttribute("aria-expanded", String(!open));
  });
  if (window.matchMedia("(max-width: 880px)").matches) {
    tabsAside.classList.add("collapsed");
    menuBtn.setAttribute("aria-expanded", "false");
  }

  window.addEventListener("hashchange", route);
  route();
})();
