// Minimal Centrifugo client (JSON protocol v2): connect with a JWT, answer
// server pings, receive pushes from server-side subscriptions, reconnect.
class Realtime {
  constructor(url, token, onPush, onState) {
    this.url = url; this.token = token; this.onPush = onPush; this.onState = onState;
    this.id = 0; this.backoff = 500; this.closed = false;
    this.open();
  }
  open() {
    if (this.closed) return;
    this.onState('connecting');
    const ws = new WebSocket(this.url);
    this.ws = ws;
    ws.onopen = () => ws.send(JSON.stringify({ id: ++this.id, connect: { token: this.token, name: 'admin-panel' } }));
    ws.onmessage = (e) => {
      for (const line of String(e.data).split('\n')) {
        if (!line) continue;
        const msg = JSON.parse(line);
        if (Object.keys(msg).length === 0) { ws.send('{}'); continue; } // ping -> pong
        if (msg.connect) { this.backoff = 500; this.onState('live'); }
        if (msg.error) console.warn('realtime error', msg.error);
        if (msg.push && msg.push.pub) this.onPush(msg.push.channel, msg.push.pub.data);
      }
    };
    ws.onclose = () => {
      this.onState('offline');
      if (this.closed) return;
      setTimeout(() => this.open(), this.backoff);
      this.backoff = Math.min(this.backoff * 2, 10000);
    };
  }
  close() { this.closed = true; if (this.ws) this.ws.close(); }
}
