#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = [
#   "msgpack>=1.1.0",
#   "requests>=2.32.0",
# ]
# ///
"""Probe the NovelAI image generation endpoint used by novelai.net/image."""

from __future__ import annotations

import argparse
import base64
import datetime as dt
import io
import json
import os
import sys
import tempfile
import threading
import uuid
import zipfile
from email import policy
from email.parser import BytesParser
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

try:
    import msgpack
    import requests
except ImportError as exc:
    raise SystemExit(
        "Missing dependencies. Run with `uv run "
        "tools/novelai_image_probe.py` or install requests and msgpack."
    ) from exc


STREAM_ENDPOINT = "https://image.novelai.net/ai/generate-image-stream"
LEGACY_ENDPOINT = "https://image.novelai.net/ai/generate-image"
DEFAULT_MODEL = "nai-diffusion-5-curated"
PNG_1X1 = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9WlYqWQAAAAASUVORK5CYII="
)


class ProbeError(RuntimeError):
    """Raised when NovelAI returns an invalid or unsuccessful response."""


def utc_now() -> str:
    return (
        dt.datetime.now(dt.timezone.utc)
        .isoformat(timespec="milliseconds")
        .replace("+00:00", "Z")
    )


def build_payload(args: argparse.Namespace) -> dict[str, Any]:
    parameters: dict[str, Any] = {
        "params_version": 4,
        "width": args.width,
        "height": args.height,
        "scale": args.scale,
        "sampler": "k_euler_ancestral",
        "steps": args.steps,
        "seed": args.seed,
        "n_samples": args.n_samples,
        "strength": 0.7,
        "noise": 0,
        "module": None,
        "ucPresetId": args.uc_preset,
        "qualityPresetId": args.quality_preset,
        "tag_hint_transparent_background": False,
        "autoSmea": False,
        "sm": False,
        "sm_dyn": False,
        "dynamic_thresholding": False,
        "controlnet_strength": 1,
        "legacy": False,
        "add_original_image": True,
        "cfg_rescale": 0,
        "noise_schedule": "karras",
        "legacy_v3_extend": False,
        "skip_cfg_above_sigma": None,
        "use_coords": False,
        "legacy_uc": False,
        "normalize_reference_strength_multiple": True,
        "inpaintImg2ImgStrength": 1,
    }
    if args.negative_prompt:
        parameters["negative_prompt"] = args.negative_prompt
    if args.mode == "stream":
        parameters["stream"] = "msgpack"

    return {
        "input": args.prompt,
        "model": args.model,
        "action": "generate",
        "parameters": parameters,
        "use_new_shared_trial": True,
    }


def build_multipart(payload: dict[str, Any], boundary: str) -> bytes:
    request_json = json.dumps(
        payload, ensure_ascii=False, separators=(",", ":")
    ).encode("utf-8")
    delimiter = boundary.encode("ascii")
    return b"".join(
        [
            b"--",
            delimiter,
            b"\r\n",
            b'Content-Disposition: form-data; name="request"; filename="blob"\r\n',
            b"Content-Type: application/json\r\n",
            b"\r\n",
            request_json,
            b"\r\n--",
            delimiter,
            b"--\r\n",
        ]
    )


def build_headers(token: str, boundary: str) -> dict[str, str]:
    return {
        "Authorization": f"Bearer {token}",
        "x-correlation-id": str(uuid.uuid4()),
        "x-initiated-at": utc_now(),
        "Origin": "https://novelai.net",
        "Referer": "https://novelai.net/image",
        "Cache-Control": "no-cache",
        "Content-Type": f"multipart/form-data; boundary={boundary}",
    }


def image_extension(data: bytes) -> str:
    if data.startswith(b"\x89PNG\r\n\x1a\n"):
        return ".png"
    if data.startswith(b"\xff\xd8\xff"):
        return ".jpg"
    if len(data) >= 12 and data[:4] == b"RIFF" and data[8:12] == b"WEBP":
        return ".webp"
    return ".bin"


def output_name(output_dir: Path, sample_index: int, data: bytes) -> Path:
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%d-%H%M%S")
    return output_dir / f"novelai-{stamp}-{sample_index}{image_extension(data)}"


def save_image(output_dir: Path, sample_index: int, data: bytes) -> Path:
    output_dir.mkdir(parents=True, exist_ok=True)
    path = output_name(output_dir, sample_index, data)
    path.write_bytes(data)
    return path


def request_stream(
    args: argparse.Namespace,
    endpoint: str,
    token: str,
    output_dir: Path,
) -> list[Path]:
    payload = build_payload(args)
    boundary = f"----novelai-{uuid.uuid4().hex}"
    headers = build_headers(token, boundary)

    try:
        response = requests.post(
            endpoint,
            headers=headers,
            data=build_multipart(payload, boundary),
            timeout=args.timeout,
            stream=True,
        )
    except requests.RequestException as exc:
        raise ProbeError(f"Request failed: {exc}") from exc

    if response.status_code >= 400:
        body = response.text[:4000]
        raise ProbeError(
            f"NovelAI returned HTTP {response.status_code}: {body}"
        )

    buffer = bytearray()
    saved: list[Path] = []
    expected = args.n_samples
    finals_seen: set[int] = set()
    intermediates_seen = 0

    for chunk in response.iter_content(chunk_size=64 * 1024):
        if not chunk:
            continue
        buffer.extend(chunk)
        while len(buffer) >= 4:
            frame_size = int.from_bytes(buffer[:4], "big")
            if len(buffer) < 4 + frame_size:
                break
            frame = bytes(buffer[4 : 4 + frame_size])
            del buffer[: 4 + frame_size]
            try:
                event = msgpack.unpackb(frame, raw=False, strict_map_key=False)
            except Exception as exc:
                raise ProbeError(f"Invalid MessagePack frame: {exc}") from exc

            event_type = event.get("event_type")
            if event_type == "error":
                raise ProbeError(
                    f"Generation error: {event.get('message', 'unknown error')}"
                )
            if event_type == "intermediate":
                intermediates_seen += 1
                if args.verbose:
                    print(
                        "intermediate "
                        f"sample={event.get('samp_ix')} "
                        f"step={event.get('step_ix')}",
                        file=sys.stderr,
                    )
                continue
            if event_type == "final":
                sample_index = int(event.get("samp_ix", 0))
                image = event.get("image")
                if not isinstance(image, (bytes, bytearray, memoryview)):
                    raise ProbeError(
                        f"Final frame {sample_index} has no image bytes"
                    )
                path = save_image(output_dir, sample_index, bytes(image))
                saved.append(path)
                finals_seen.add(sample_index)

    if len(finals_seen) < expected:
        raise ProbeError(
            f"Stream ended after {len(finals_seen)}/{expected} final images "
            f"({intermediates_seen} intermediate frames)"
        )
    return saved


def request_legacy(
    args: argparse.Namespace,
    endpoint: str,
    token: str,
    output_dir: Path,
) -> list[Path]:
    payload = build_payload(args)
    boundary = f"----novelai-{uuid.uuid4().hex}"
    headers = build_headers(token, boundary)

    try:
        response = requests.post(
            endpoint,
            headers=headers,
            data=build_multipart(payload, boundary),
            timeout=args.timeout,
        )
    except requests.RequestException as exc:
        raise ProbeError(f"Request failed: {exc}") from exc

    if response.status_code >= 400:
        raise ProbeError(
            f"NovelAI returned HTTP {response.status_code}: "
            f"{response.text[:4000]}"
        )

    content = response.content
    saved: list[Path] = []
    if content.startswith(b"PK\x03\x04"):
        with zipfile.ZipFile(io.BytesIO(content)) as archive:
            image_names = [
                name
                for name in archive.namelist()
                if Path(name).suffix.lower()
                in {".png", ".jpg", ".jpeg", ".webp"}
            ]
            for index, name in enumerate(sorted(image_names)):
                saved.append(
                    save_image(output_dir, index, archive.read(name))
                )
    elif image_extension(content) != ".bin":
        saved.append(save_image(output_dir, 0, content))
    else:
        output_dir.mkdir(parents=True, exist_ok=True)
        raw_path = output_dir / "novelai-response.bin"
        raw_path.write_bytes(content)
        raise ProbeError(
            f"Unsupported response format; raw response saved to {raw_path}"
        )

    if len(saved) < args.n_samples:
        raise ProbeError(
            f"Response contained {len(saved)}/{args.n_samples} images"
        )
    return saved


def print_dry_run(args: argparse.Namespace, endpoint: str) -> None:
    payload = build_payload(args)
    boundary = "----novelai-dry-run"
    headers = build_headers("<NOVELAI_TOKEN>", boundary)
    print(f"POST {endpoint}")
    print(json.dumps(headers, ensure_ascii=False, indent=2))
    print("\nrequest part:")
    print(json.dumps(payload, ensure_ascii=False, indent=2))


def self_test() -> None:
    received: dict[str, Any] = {}

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:
            length = int(self.headers.get("Content-Length", "0"))
            body = self.rfile.read(length)
            content_type = self.headers.get("Content-Type", "")
            envelope = (
                f"Content-Type: {content_type}\r\n"
                "MIME-Version: 1.0\r\n\r\n"
            ).encode()
            message = BytesParser(policy=policy.default).parsebytes(
                envelope + body
            )
            part = message.get_payload()[0]
            received.update(
                {
                    "path": self.path,
                    "authorization": self.headers.get("Authorization"),
                    "correlation_id": self.headers.get("x-correlation-id"),
                    "initiated_at": self.headers.get("x-initiated-at"),
                    "payload": json.loads(part.get_payload(decode=True)),
                }
            )
            frame = msgpack.packb(
                {
                    "event_type": "final",
                    "image": PNG_1X1,
                    "samp_ix": 0,
                    "step_ix": 1,
                },
                use_bin_type=True,
            )
            response = len(frame).to_bytes(4, "big") + frame
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)

        def log_message(self, _format: str, *args: Any) -> None:
            return

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix="novelai-probe-") as temp:
            args = argparse.Namespace(
                prompt="self test",
                negative_prompt="",
                model=DEFAULT_MODEL,
                mode="stream",
                width=64,
                height=64,
                steps=1,
                scale=7.0,
                seed=1,
                n_samples=1,
                uc_preset="heavy",
                quality_preset="standard",
                timeout=10,
                verbose=False,
            )
            port = server.server_address[1]
            paths = request_stream(
                args,
                f"http://127.0.0.1:{port}/ai/generate-image-stream",
                "test-token",
                Path(temp),
            )
            assert received["path"] == "/ai/generate-image-stream"
            assert received["authorization"] == "Bearer test-token"
            assert received["correlation_id"]
            assert received["initiated_at"]
            assert received["payload"]["input"] == "self test"
            assert (
                received["payload"]["parameters"]["stream"] == "msgpack"
            )
            assert len(paths) == 1
            assert paths[0].read_bytes() == PNG_1X1
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
    print("self-test: ok")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Test NovelAI streaming or legacy image generation."
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run against a local mock server without using an account.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the request without sending it.",
    )
    parser.add_argument(
        "--mode",
        choices=("stream", "legacy"),
        default="stream",
        help="Use the streaming endpoint or the legacy ZIP endpoint.",
    )
    parser.add_argument("--prompt", default="a simple blue circle")
    parser.add_argument("--negative-prompt", default="")
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--width", type=int, default=832)
    parser.add_argument("--height", type=int, default=1216)
    parser.add_argument("--steps", type=int, default=23)
    parser.add_argument("--scale", type=float, default=7.0)
    parser.add_argument("--seed", type=int)
    parser.add_argument("--n-samples", type=int, default=1)
    parser.add_argument("--uc-preset", default="heavy")
    parser.add_argument("--quality-preset", default="standard")
    parser.add_argument("--timeout", type=float, default=180.0)
    parser.add_argument(
        "--endpoint",
        help="Override the endpoint URL. Useful for local mock testing.",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("tmp/novelai-probe"),
        help="Directory for generated images.",
    )
    parser.add_argument("--verbose", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.self_test:
        self_test()
        return 0

    endpoint = args.endpoint or (
        STREAM_ENDPOINT if args.mode == "stream" else LEGACY_ENDPOINT
    )
    if args.dry_run:
        print_dry_run(args, endpoint)
        return 0

    token = os.environ.get("NOVELAI_TOKEN")
    if not token:
        raise SystemExit(
            "NOVELAI_TOKEN is not set. Use --dry-run or --self-test, "
            "or export NOVELAI_TOKEN first."
        )

    try:
        if args.mode == "stream":
            paths = request_stream(args, endpoint, token, args.output)
        else:
            paths = request_legacy(args, endpoint, token, args.output)
    except ProbeError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    for path in paths:
        print(path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
