from urllib.parse import parse_qs, urlparse
import asyncio
from unittest.mock import AsyncMock, Mock

from app import main
from app.main import capture_url


def test_capture_url_preserves_existing_query_and_enables_complete_mode():
    url = capture_url("http://sub2api:8080/model-status?group_id=2", "gemini")
    parsed = urlparse(url)

    assert parsed.path == "/model-status"
    assert parse_qs(parsed.query) == {"group_id": ["2"], "search": ["gemini"], "capture": ["all"]}


def test_capture_url_does_not_duplicate_capture_parameter():
    url = capture_url("http://sub2api:8080/model-status?capture=all")

    assert url.count("capture=all") == 1


def test_capture_url_replaces_old_search_value_and_omits_empty_search():
    url = capture_url("http://sub2api:8080/model-status?search=old", "  Claude  ")
    assert parse_qs(urlparse(url).query)["search"] == ["Claude"]

    url = capture_url("http://sub2api:8080/model-status?search=old", " ")
    assert "search" not in parse_qs(urlparse(url).query)


def test_screenshot_waits_for_complete_filtered_page(monkeypatch):
    page = Mock()
    page.goto = AsyncMock()
    ready = Mock(wait_for=AsyncMock())
    page.locator.return_value = ready
    page.screenshot = AsyncMock(return_value=b"png")
    page.close = AsyncMock()
    browser = Mock(new_page=AsyncMock(return_value=page))
    monkeypatch.setattr(main, "_browser", browser)
    monkeypatch.setattr(main, "PAGE_URL", "http://sub2api:8080/model-status")

    response = asyncio.run(main.screenshot("Pro稳定"))

    query = parse_qs(urlparse(page.goto.call_args.args[0]).query)
    assert query == {"capture": ["all"], "search": ["Pro稳定"]}
    page.locator.assert_called_once_with('[data-testid="model-status-ready"]')
    ready.wait_for.assert_awaited_once_with(state="attached", timeout=30000)
    page.screenshot.assert_awaited_once_with(type="png", full_page=True, animations="disabled")
    page.close.assert_awaited_once()
    assert response.body == b"png"
    assert response.headers["cache-control"] == "no-store"
