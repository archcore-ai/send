package api

import (
	"net/http"
	"strings"
)

// handleLanding serves a static page at "/" so anyone can discover what this
// instance is and how to install the skill. Anchored as GET /{$} so it never
// shadows /s/{id}, /v1/..., or /healthz.
func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	base := s.publicBase(r)
	page := strings.ReplaceAll(landingHTML, "{{BASE}}", base)
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

const landingHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Archcore Send</title>
<style>
  body { font: 16px/1.6 system-ui, sans-serif; max-width: 44rem; margin: 4rem auto; padding: 0 1.25rem; color: #1a1a1a; }
  code, pre { background: #f4f4f5; border-radius: 6px; }
  code { padding: .1rem .35rem; }
  pre { padding: 1rem; overflow-x: auto; }
  h1 { margin-bottom: .25rem; }
  .muted { color: #666; }
  a { color: #2563eb; }
</style>
</head>
<body>
<h1>Archcore Send</h1>
<p class="muted">End-to-end-encrypted context handoff for AI coding sessions.</p>
<p>This server is a <strong>zero-knowledge ciphertext rendezvous</strong>. It stores only
encrypted parts and never sees plaintext or the decryption key — the key lives in the
link fragment and is parsed locally by the client.</p>
<h2>Use it</h2>
<p>Install the <code>send</code> skill, then point it at this server:</p>
<pre><code>export SEND_SERVER_URL={{BASE}}
# package the current session and get a one-time link
/send
# load a link someone shared with you
/send --load &lt;url&gt;</code></pre>
<p>Check your environment first with <code>/send --doctor</code> (needs <code>age</code> installed).</p>
<p class="muted">Links are one-time and short-lived. Treat the full link (with <code>#agekey=…</code>)
like a secret — anyone with it can decrypt.</p>
</body>
</html>`

const linkHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Archcore Send — encrypted link</title>
<style>
  body { font: 16px/1.6 system-ui, sans-serif; max-width: 40rem; margin: 4rem auto; padding: 0 1.25rem; color: #1a1a1a; }
  code { background: #f4f4f5; border-radius: 6px; padding: .1rem .35rem; }
  .muted { color: #666; }
</style>
</head>
<body>
<h1>Encrypted send</h1>
<p>This is an end-to-end-encrypted Archcore Send link. Open it with the <code>send</code> skill:</p>
<p><code>/send --load &lt;this full url&gt;</code></p>
<p class="muted">The decryption key is in the part of the URL after <code>#</code>; it never reaches
this server. Links are one-time and expire.</p>
</body>
</html>`
