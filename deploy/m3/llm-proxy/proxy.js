// llm-proxy v3: normalizes claude-code gateway traffic for openrouter
// - answers /api/hello health pings (200 {})
// - injects provider pinning so the anthropic-compat translation is deterministic
// - SANITIZE: deletes the non-standard `is_error` field from tool_result content
//   blocks (OpenRouter 422 "no such field: 'is_error'"); recursive over messages
// - forwards everything else to https://openrouter.ai
const http = require('http');
const https = require('https');

function sanitize(obj) {
  if (Array.isArray(obj)) { obj.forEach(sanitize); return obj; }
  if (obj && typeof obj === 'object') {
    if (obj.type === 'tool_result' && 'is_error' in obj) delete obj.is_error;
    Object.values(obj).forEach(sanitize);
  }
  return obj;
}

const H = http.createServer((req, res) => {
  let chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const body = Buffer.concat(chunks);
    const reply = (code, text) => { try { res.writeHead(code, {'content-type':'application/json'}); res.end(text); } catch(_) {} };
    if (req.url.includes('/hello')) return reply(200, '{}');
    let out = body;
    try {
      const j = JSON.parse(body.toString());
      if (j.model && !j.provider) j.provider = { order: ['friendli'], allow_fallbacks: false };
      if (Array.isArray(j.messages)) sanitize(j.messages);
      out = Buffer.from(JSON.stringify(j));
    } catch (e) {}
    const up = https.request('https://openrouter.ai' + req.url, {
      method: req.method,
      headers: { 'content-type': 'application/json', 'x-api-key': req.headers['x-api-key'], 'anthropic-version': req.headers['anthropic-version'], 'accept': req.headers['accept'] }
    }, ur => {
      res.writeHead(ur.statusCode, { 'content-type': ur.headers['content-type'] || 'application/json' });
      ur.pipe(res);
    });
    up.on('error', e => reply(502, JSON.stringify({error: String(e).slice(0,200)})));
    up.end(out);
  });
});
H.listen(9999, () => console.log('llm-proxy v3 up (sanitize)'));
