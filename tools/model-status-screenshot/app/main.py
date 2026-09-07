import os
from contextlib import asynccontextmanager
from typing import TYPE_CHECKING, Any
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

from fastapi import FastAPI, HTTPException
from fastapi.responses import Response

if TYPE_CHECKING:
    from playwright.async_api import Browser, Playwright


PAGE_URL = os.getenv("MODEL_STATUS_PAGE_URL", "http://sub2api:8080/model-status")
CHROMIUM_EXECUTABLE = os.getenv("CHROMIUM_EXECUTABLE", "/usr/bin/chromium")
_playwright: Any = None
_browser: Any = None


def capture_url(url: str) -> str:
    """Ensure the page renders every group/model for a complete screenshot."""
    parts = urlsplit(url)
    query = [(key, value) for key, value in parse_qsl(parts.query, keep_blank_values=True) if key != "capture"]
    query.append(("capture", "all"))
    return urlunsplit((parts.scheme, parts.netloc, parts.path, urlencode(query), parts.fragment))


@asynccontextmanager
async def lifespan(_: FastAPI):
    global _playwright, _browser
    from playwright.async_api import async_playwright

    _playwright = await async_playwright().start()
    _browser = await _playwright.chromium.launch(
        executable_path=CHROMIUM_EXECUTABLE,
        headless=True,
        args=["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"],
        env={"HOME": "/tmp", "XDG_CONFIG_HOME": "/tmp/chromium-config", "XDG_CACHE_HOME": "/tmp/chromium-cache"},
    )
    try:
        yield
    finally:
        await _browser.close()
        await _playwright.stop()
        _browser = None
        _playwright = None


app = FastAPI(title="Sub2API model status screenshot", lifespan=lifespan)


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/screenshot")
async def screenshot():
    if _browser is None:
        raise HTTPException(status_code=503, detail="renderer not ready")

    page = await _browser.new_page(viewport={"width": 1600, "height": 900}, device_scale_factor=1)
    try:
        await page.goto(capture_url(PAGE_URL), wait_until="domcontentloaded", timeout=30000)
        await page.locator('[data-testid="model-status-ready"]').wait_for(state="attached", timeout=30000)
        image = await page.screenshot(type="png", full_page=True, animations="disabled")
        return Response(content=image, media_type="image/png", headers={"Cache-Control": "no-store"})
    except Exception as exc:
        raise HTTPException(status_code=503, detail="renderer failed") from exc
    finally:
        await page.close()
