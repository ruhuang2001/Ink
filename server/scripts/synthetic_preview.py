#!/usr/bin/env python3
"""Read-only account/API synthetic: render a PNG, never create or submit jobs."""

import base64
import json
import os
import struct
import time
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    base = os.environ["INK_SYNTHETIC_API_URL"].rstrip("/")
    token = os.environ["INK_SYNTHETIC_ACCESS_TOKEN"]
    url = urllib.parse.urlsplit(base)
    if url.username or url.password or url.query or url.fragment or not url.hostname:
        raise ValueError("Invalid API origin")
    if url.scheme != "https" and not (url.scheme == "http" and url.hostname in ("127.0.0.1", "localhost")):
        raise ValueError("Synthetic requires HTTPS or local loopback HTTP")
    opener = urllib.request.build_opener(NoRedirect())
    results = []
    # Fixed allowlist deliberately excludes create, submit, retry and device routes.
    for path, payload in [("/healthz", None), ("/readyz", None), ("/api/v1/workspace", None),
                          ("/api/v1/print-preview", {"title": "Synthetic preview", "content": "Ink synthetic monitoring: preview only, no printing."})]:
        data = None if payload is None else json.dumps(payload).encode()
        request = urllib.request.Request(base + path, data=data, headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"})
        started = time.monotonic()
        with opener.open(request, timeout=30) as response:
            body = json.loads(response.read())
            if response.status != 200:
                raise RuntimeError("Unexpected synthetic status")
        if payload is not None:
            image = body["image"]
            raw = base64.b64decode(image, validate=True)
            if raw[:8] != b"\x89PNG\r\n\x1a\n" or len(raw) < 24:
                raise RuntimeError("Preview did not return PNG")
            width, height = struct.unpack(">II", raw[16:24])
            if width != 384 or not 0 < height <= 8192:
                raise RuntimeError("Unexpected preview dimensions")
        results.append({"route": path, "duration_ms": round((time.monotonic()-started)*1000, 2)})
    print(json.dumps({"schema_version": 1, "success": True, "checks": results}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception:
        # Errors/URLs/tokens and account content must not enter monitoring logs.
        print(json.dumps({"schema_version": 1, "success": False, "error": "synthetic_preview_failed"}))
        raise SystemExit(1)
