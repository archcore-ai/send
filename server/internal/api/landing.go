package api

import (
	"net/http"
	"strings"
)

// defaultPublicServer is the built-in default the client falls back to when
// SEND_SERVER_URL is unset. On this host the export line is redundant, so the
// landing page shows the zero-config path instead.
const defaultPublicServer = "https://send.archcore.ai"

// handleLanding serves a static page at "/" so anyone can discover what this
// instance is and how to install the skill. Anchored as GET /{$} so it never
// shadows /s/{id}, /v1/..., or /healthz.
func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	base := s.publicBase(r)
	use := useSelfhostBlock
	if base == defaultPublicServer {
		use = useDefaultBlock
	}
	page := strings.ReplaceAll(landingHTML, "{{USE}}", use)
	page = strings.ReplaceAll(page, "{{BASE}}", base)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(page))
}

// handleLinkPage serves a cosmetic, no-secret page for a public send link
// (/s/{id}). The actual load happens locally via the skill; the decryption key
// lives in the URL fragment and never reaches this server.
func (s *Server) handleLinkPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(linkHTML))
}

// pageStyle is the shared, dependency-free visual system (Archcore DESIGN.md:
// warm cream + subtle grid, black text/actions, warm-paper code). No web fonts —
// system stacks only, to keep the page light and instant.
const pageStyle = `<style>
  :root {
    --page:#f8f1e8; --surface:#fbf7ee; --paper:#f5eee1;
    --text:#11100e; --secondary:#5f5a50; --muted:#8a8377;
    --border:#d5c8b6; --border-subtle:#e3d8c8;
  }
  * { box-sizing: border-box; }
  body {
    font: 15px/1.6 Inter, ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
    color: var(--text);
    background-color: var(--page);
    background-image:
      linear-gradient(rgba(17,16,14,.035) 1px, transparent 1px),
      linear-gradient(90deg, rgba(17,16,14,.035) 1px, transparent 1px);
    background-size: 32px 32px;
    margin: 0; padding: 88px 20px;
  }
  main { max-width: 680px; margin: 0 auto; }
  h1 { font-size: 34px; line-height: 1.05; font-weight: 700; letter-spacing: -.035em; margin: 0 0 .5rem; }
  h2 { font-size: 18px; font-weight: 650; letter-spacing: -.02em; margin: 2.25rem 0 .6rem; }
  p { margin: 0 0 1rem; max-width: 62ch; }
  strong { font-weight: 650; }
  .lede { color: var(--secondary); font-size: 16px; margin-bottom: 1.75rem; }
  .muted { color: var(--muted); font-size: 13px; }
  .badge {
    display: inline-block; margin-bottom: 1.25rem;
    font-size: 11px; font-weight: 600; letter-spacing: .02em;
    color: var(--secondary); background: var(--surface);
    border: 1px solid var(--border); border-radius: 999px; padding: 3px 9px;
  }
  code {
    font-family: "JetBrains Mono", ui-monospace, SFMono-Regular, Consolas, monospace;
    font-size: .88em; background: var(--surface);
    border: 1px solid var(--border-subtle); border-radius: 6px; padding: .08rem .32rem;
  }
  pre {
    background: var(--paper); border: 1px solid var(--border); border-radius: 12px;
    padding: 14px 16px; overflow-x: auto; margin: 0 0 1rem;
  }
  pre code { background: none; border: 0; padding: 0; font-size: 13px; line-height: 1.6; color: var(--text); }
  .comment { color: var(--muted); }
  a { color: var(--text); text-underline-offset: 3px; }
  .footer {
    margin-top: 2.75rem; padding-top: 1.25rem;
    border-top: 1px solid var(--border-subtle);
    font-size: 13px; color: var(--muted);
  }
  .footer a { color: var(--secondary); }
  .footer .sep { color: var(--border); padding: 0 .4rem; }
  :focus-visible { outline: 2px solid #11100e; outline-offset: 3px; }
  @media (max-width: 600px) { body { padding: 56px 20px; } h1 { font-size: 30px; } }
</style>`

const landingHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Archcore Send</title>
` + pageStyle + `
</head>
<body>
<main>
<span class="badge">Zero-knowledge</span>
<h1>Archcore Send</h1>
<p class="lede">Hand off an AI coding session — encrypted, one-time, zero-knowledge.</p>
<p>Package a session's context, encrypt it locally, share a one-time link. This server
stores only ciphertext — never plaintext, never the key.</p>
<h2>Use it</h2>
{{USE}}
<p class="muted">The full link — including its <code>#agekey=…</code> fragment — is a secret:
anyone who has it can decrypt.</p>
<div class="footer">
<a href="https://github.com/archcore-ai/send">Docs &amp; source on GitHub</a>
<span class="sep">·</span>
Built by <a href="https://github.com/ivklgn">@ivklgn</a>
</div>
</main>
</body>
</html>`

// useDefaultBlock — shown when this instance IS the built-in default
// (send.archcore.ai). The skill already targets it, so no export is needed.
const useDefaultBlock = `<p>Install the <a href="https://github.com/archcore-ai/send#install-the-skill">send skill</a> in your agent — it talks to this server out of the box:</p>
<pre><code>/send
/send --load &lt;url&gt;</code></pre>`

// useSelfhostBlock — shown for any other (self-hosted) instance, where the
// client must be pointed at this server explicitly.
const useSelfhostBlock = `<pre><code><span class="comment"># install the skill, then point it at this server</span>
cp -R skill/send ~/.claude/skills/send
export SEND_SERVER_URL={{BASE}}
/send
/send --load &lt;url&gt;</code></pre>`

const linkHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Archcore Send — encrypted link</title>
` + pageStyle + `
</head>
<body>
<main>
<span class="badge">Encrypted link</span>
<h1>Encrypted send</h1>
<p>This is an end-to-end-encrypted Archcore Send link. Open it with the <code>send</code> skill:</p>
<pre><code>/send --load &lt;this full url&gt;</code></pre>
<p class="muted">The decryption key is the part of the URL after <code>#</code> — it never reaches
this server. Links are one-time and expire.</p>
<div class="footer">
<a href="https://github.com/archcore-ai/send">More info &amp; source on GitHub</a>
<span class="sep">·</span>
Built by <a href="https://github.com/ivklgn">@ivklgn</a>
</div>
</main>
</body>
</html>`
