# tsproxy

An HTTPS-adding proxy for Tailscale.

Say you have a program listening on a Unix domain socket at `~/server.sock`.
You can expose it, as a valid HTTPS `http://my-little-server.tailename-scalename.ts.net`,
with:

```
tsproxy \
  --from my-little-server \
  --to ~/server.sock
```

## Hey, that didn't work!

The first time you start up `tsproxy`, you need to prove an [Auth
key](https://tailscale.com/kb/1085/auth-keys/) in the `TS_AUTHKEY` environment
variable, e.g.:

```
TS_AUTHKEY=tskey-asdf-jklsemicolon \
  tsproxy \
    --from my-little-server \
    --to ~/server.sock
```

If you use a One-of Key to authenticate, future invocations of `tsproxy` for the
same `--from` address will already be authenticated.

## Why?

I wrote `tsproxy` while working on a [web app] project, using [code-server] as
my IDE, on a different computer than my hands (desktop vs. Chromebook).

[code-server]: https://github.com/coder/code-server
[web app]: https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps

Utilmately, I wanted to expose the web app through my tailnet; and while
working, I liked having the "launch an app" UI for code-server. Installing a
page as an "app" requires that the page is served over HTTPS. (Or via `localhost`-
but I didn't want to tunnel over SSH unnecessarily.)

Tailscale's built-in TLS credentials feature - "get credentials via `tailscale
creds`" - worked for this, but not quite in the way I wanted it to. Each "app"
was on a different port, rather than living on the default (443); and all of
them had the same set of credentials, for the host. Getting those
credentials, as far as I can tell, required running `tailscale creds` as a
privileged user.

Instead, `tsproxy` uses the (experimental!) `tsnet` userspace client for
Tailscale- not rerouting all traffic from the node, only rerouting traffic from
its listener.

## Don't use this

You'll note that this doesn't have much in the way of tests, documentation,
fuzzing, or updates. Use at our own risl.
