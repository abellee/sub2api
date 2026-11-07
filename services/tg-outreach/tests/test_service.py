import asyncio
import json
import time
from datetime import datetime, timezone
from types import SimpleNamespace
from unittest.mock import AsyncMock

import httpx
import pytest
import pytest_asyncio
from fastapi.testclient import TestClient
from telethon import errors, functions, types, utils

from engine import Engine, error_message
from main import create_app
from models import ModelAPI, Settings, UserError, api_root, keywords, parse_selection, render_prompt
from store import Store

KEY = "test-admin-secret-with-at-least-24-characters"
HASH = "a" * 32


def channel(ident, title="测试群", username="testgroup", **kwargs):
    return types.Channel(id=ident, title=title, photo=types.ChatPhotoEmpty(),
        date=datetime.now(timezone.utc), access_hash=ident + 100, megagroup=True, username=username, **kwargs)


class FakeTelegram:
    def __init__(self, *args, **kwargs):
        self.connected = True
        self.dialogs = []
        self.chats = []
        self.joined = []
        self.sent = []
        self.authorized = False
        self.require_password = False
        self.send_error = self.join_error = None
        self.session = SimpleNamespace(delete=lambda: None)

    def is_connected(self):
        return self.connected

    async def connect(self):
        self.connected = True

    async def disconnect(self):
        self.connected = False

    async def is_user_authorized(self):
        return self.authorized

    async def get_me(self):
        return SimpleNamespace(first_name="测试", last_name="账号", username="tester")

    async def iter_dialogs(self):
        for chat in self.dialogs:
            yield SimpleNamespace(entity=chat, is_group=bool(chat.megagroup), name=chat.title)

    async def send_code_request(self, phone):
        return SimpleNamespace(phone_code_hash="private-code-hash")

    async def sign_in(self, **kwargs):
        if self.require_password and "password" not in kwargs:
            raise errors.SessionPasswordNeededError(request=None)
        self.authorized = True

    async def log_out(self):
        self.authorized = False
        return True

    async def send_message(self, ident, message, **kwargs):
        if self.send_error:
            raise self.send_error
        self.sent.append((ident, message, kwargs))

    async def get_entity(self, username):
        return next(chat for chat in self.chats if chat.username == username)

    async def __call__(self, request):
        if isinstance(request, functions.contacts.SearchRequest):
            return SimpleNamespace(chats=self.chats)
        if isinstance(request, functions.channels.JoinChannelRequest):
            if self.join_error:
                raise self.join_error
            self.joined.append(request.channel.id)
            self.dialogs.append(request.channel)
            return None
        raise AssertionError(type(request))


@pytest_asyncio.fixture
async def engine(tmp_path):
    store = Store(tmp_path)
    store.save_config(Settings(telegram_api_id=123, telegram_api_hash=HASH, message="推广消息",
        keywords="Python, AI", api_base="https://example.test/v1", model="test-model",
        selected_group_ids=[str(utils.get_peer_id(channel(1))), str(utils.get_peer_id(channel(2)))]).model_dump())
    engine = Engine(store, tmp_path, client_factory=FakeTelegram)
    await engine.connect()
    engine.me = await engine.client.get_me()
    yield engine
    await engine.close()
    store.db.close()


@pytest.fixture
def web(tmp_path):
    with TestClient(create_app(tmp_path, KEY)) as client:
        yield client


def login(client):
    response = client.post("/api/login", json={"key": KEY})
    assert response.status_code == 200
    return response


def test_admin_authentication_and_secret_redaction(web):
    assert web.get("/api/status").status_code == 401
    response = login(web)
    assert "HttpOnly" in response.headers["set-cookie"]
    assert "SameSite=strict" in response.headers["set-cookie"]
    assert web.put("/api/settings", json={"telegram_api_id": 123, "telegram_api_hash": HASH,
        "api_key": "secret-api-key", "message": "hello"}).status_code == 200
    settings = web.get("/api/settings").json()
    assert settings["api_key"] == settings["telegram_api_hash"] == ""
    assert settings["api_key_saved"] and settings["telegram_api_hash_saved"]
    assert web.put("/api/settings", json={"telegram_api_id": 123, "message": "changed"}).status_code == 200
    store = Store(web.app.state.engine.directory)
    try:
        assert store.config()["api_key"] == "secret-api-key"
        assert store.config()["telegram_api_hash"] == HASH
    finally:
        store.db.close()
    assert "secret-api-key" not in web.get("/api/status").text
    assert web.post("/api/logout").status_code == 200
    assert web.get("/api/settings").status_code == 401


def test_login_rate_limit_and_csrf(web):
    assert web.post("/api/login", headers={"Origin": "https://evil.test"}, json={"key": KEY}).status_code == 403
    for _ in range(5):
        assert web.post("/api/login", json={"key": "wrong"}).status_code == 401
    assert web.post("/api/login", json={"key": KEY}).status_code == 429


def test_validation_never_echoes_secrets(web):
    login(web)
    secret = "sensitive-invalid-telegram-hash"
    response = web.put("/api/settings", json={"telegram_api_hash": secret, "send_interval": 0})
    assert response.status_code == 422
    assert secret not in response.text
    assert web.post("/api/tasks/send", json={"enabled": True}).status_code == 400


def test_static_assets_and_preview(web):
    assert web.get("/").status_code == 200
    for name in ("style.css", "app.js"):
        response = web.get("/assets/" + name)
        assert response.status_code == 200
        assert response.headers["x-frame-options"] == "DENY"
    assert web.get("/assets/state.sqlite3").status_code == 404
    assert web.get("/health").json() == {"ok": True, "version": "1.0.0"}
    login(web)
    prompt = web.post("/api/prompt-preview").json()["prompt"]
    assert "{{results}}" not in prompt and "示例群" in prompt


def test_persistence_and_explicit_task_controls(tmp_path):
    with TestClient(create_app(tmp_path, KEY)) as client:
        login(client)
        client.put("/api/settings", json={"message": "persistent", "send_enabled": True})
        assert not client.get("/api/status").json()["send_enabled"]
    store = Store(tmp_path)
    store.set_runtime("cooldown", time.time() + 500)
    store.db.close()
    with TestClient(create_app(tmp_path, KEY)) as client:
        login(client)
        assert client.get("/api/settings").json()["message"] == "persistent"
        assert client.get("/api/status").json()["cooldown_until"] > time.time()


@pytest.mark.parametrize("address,expected", [
    ("https://a.test", "https://a.test/v1"),
    ("https://a.test/v1/", "https://a.test/v1"),
    ("https://a.test/compat/v1/chat/completions", "https://a.test/compat/v1"),
    ("http://localhost:8080/models", "http://localhost:8080/v1"),
])
def test_api_root(address, expected):
    assert api_root(address) == expected


@pytest.mark.parametrize("address", ["file:///etc/passwd", "https://user:pass@a.test", "https://a.test?key=secret", "https://a.test/#x"])
def test_reject_unsafe_api_syntax(address):
    with pytest.raises(UserError):
        api_root(address)


def test_template_is_replaced_once_and_keywords_deduplicated():
    candidates = [{"id": "1", "title": "{{keyword}} should remain literal"}]
    prompt = render_prompt("{{keyword}}\n{{results}}", "AI", candidates)
    assert prompt.startswith("AI\n")
    assert "{{keyword}} should remain literal" in prompt
    assert keywords("AI，Python\nAI,  中文 ") == ["AI", "Python", "中文"]


@pytest.mark.parametrize("content", ['{"join":["999"]}', '{"join":[true]}', '{"join":"1"}', 'Not JSON', '["1"]'])
def test_model_cannot_join_unknown_groups(content):
    with pytest.raises(UserError):
        parse_selection(content, [{"id": "1", "username": "allowed"}])


def test_parse_json_and_fences():
    candidates = [{"id": "123", "title": "A"}, {"id": "456", "title": "B"}]
    selected, reason = parse_selection('```json\n{"join":[123,"123"],"reason":"主题相关"}\n```', candidates)
    assert selected == [candidates[0]] and reason == "主题相关"
    assert parse_selection('{"join":[]}', candidates)[0] == []


@pytest.mark.asyncio
async def test_real_model_http_adapter():
    requests = []

    def handler(request):
        requests.append(request)
        assert request.headers["authorization"] == "Bearer private-key"
        if request.url.path.endswith("/models"):
            return httpx.Response(200, json={"data": [{"id": "model-b"}, {"id": "model-a"}, {"id": "model-a"}]})
        payload = json.loads(request.content)
        assert payload["model"] == "model-a" and not payload["stream"]
        assert '"id": "1"' in payload["messages"][1]["content"]
        return httpx.Response(200, json={"choices": [{"message": {"content": '{"join":["1"]}'}}]})

    api = ModelAPI()
    await api.client.aclose()
    api.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    config = Settings(api_base="https://example.test/v1/chat/completions", api_key="private-key", model="model-a")
    try:
        assert await api.models(config) == ["model-a", "model-b"]
        chosen, _, prompt = await api.select(config, "AI", [{"id": "1", "title": "AI群"}])
        assert chosen[0]["id"] == "1" and "{{results}}" not in prompt
        assert len(requests) == 2
    finally:
        await api.client.aclose()


@pytest.mark.asyncio
async def test_telegram_login_two_factor_and_logout(engine):
    engine.me = None
    engine.client.require_password = True
    await engine.send_code("+8613800000000")
    assert await engine.sign_in("12345", "") == {"password_required": True}
    assert engine.me is None
    assert await engine.sign_in("", "private-password") == {"password_required": False}
    assert engine.me and engine.phone_hash is None
    engine.client.dialogs = [channel(1)]
    await engine.refresh_groups()
    engine.enable("send", True)
    await engine.logout()
    assert engine.me is None and not engine.config.send_enabled


@pytest.mark.asyncio
async def test_groups_exclude_broadcast_channels_and_send_round_robin(engine):
    first, second, broadcast = channel(1), channel(2), channel(3)
    broadcast.megagroup = False
    engine.client.dialogs = [first, second, broadcast]
    await engine.refresh_groups()
    assert engine.status()["group_count"] == 2
    engine.enable("send", True)
    await engine.send_once()
    await engine.send_once()
    await engine.send_once()
    expected = [int(item["id"]) for item in engine.groups]
    assert [sent[0] for sent in engine.client.sent] == expected + [expected[0]]
    assert engine.client.sent[0][2] == {"parse_mode": None, "link_preview": False}
    engine.enable("send", False)
    await engine.send_once()
    assert len(engine.client.sent) == 3


@pytest.mark.asyncio
async def test_only_selected_groups_receive_messages_and_new_groups_stay_unselected(engine):
    engine.client.dialogs = [channel(1), channel(2), channel(3)]
    await engine.refresh_groups()
    selected = str(utils.get_peer_id(channel(2)))
    config = engine.config.model_dump()
    config["selected_group_ids"] = [selected]
    engine.store.save_config(config)
    engine.enable("send", True)
    await engine.send_once()
    await engine.send_once()
    assert [message[0] for message in engine.client.sent] == [int(selected), int(selected)]
    assert engine.status()["group_count"] == 3
    engine.client.dialogs = [channel(1), channel(3)]
    await engine.refresh_groups()
    await engine.send_once()
    assert len(engine.client.sent) == 2


@pytest.mark.asyncio
async def test_empty_selection_cannot_start_sending(engine):
    engine.client.dialogs = [channel(1)]
    await engine.refresh_groups()
    config = engine.config.model_dump()
    config["selected_group_ids"] = []
    engine.store.save_config(config)
    with pytest.raises(UserError, match="勾选"):
        engine.enable("send", True)
    assert not engine.config.send_enabled


def test_selected_groups_persist_without_enabling_tasks(web):
    login(web)
    selected = [str(utils.get_peer_id(channel(2)))]
    assert web.put("/api/settings", json={"selected_group_ids": selected}).status_code == 200
    assert web.get("/api/settings").json()["selected_group_ids"] == selected
    assert web.put("/api/settings", json={"selected_group_ids": ["https://t.me/unknown"]}).status_code == 422


@pytest.mark.asyncio
async def test_flood_wait_is_global_and_persisted(engine):
    engine.client.dialogs = [channel(1)]
    await engine.refresh_groups()
    engine.enable("send", True)
    engine.client.send_error = errors.FloodWaitError(request=None, capture=80)
    with pytest.raises(errors.FloodWaitError) as caught:
        await engine.send_once()
    engine.record_error("发送", caught.value)
    assert engine.cooldown >= time.time() + 79
    assert engine.store.runtime("cooldown") == engine.cooldown
    with pytest.raises(UserError):
        await engine.refresh_groups()
    assert engine.store.runtime("send_cursor") is None


@pytest.mark.asyncio
async def test_search_select_join_and_ignore_existing_and_channels(engine):
    existing, chosen, broadcast = channel(1, username="old"), channel(2, username="new"), channel(3, username="news")
    broadcast.megagroup = False
    engine.client.dialogs = [existing]
    engine.client.chats = [existing, chosen, broadcast]
    engine.api.select = AsyncMock(return_value=([{"id": "2", "title": chosen.title, "username": "new"}], "相关", "完整提示词"))
    engine.enable("search", True)
    await engine.search_once()
    assert engine.client.joined == [2]
    assert engine.status()["group_count"] == 2
    assert engine.store.candidate_status("2") == "joined"
    assert engine.store.runtime("next_join") > time.time()
    assert engine.api.select.call_args.args[1] == "Python"
    assert len(engine.api.select.call_args.args[2]) == 1
    assert engine.last_prompt == "完整提示词"


@pytest.mark.asyncio
async def test_pending_join_is_not_resubmitted(engine):
    chosen = channel(2, username="approval")
    engine.client.chats = [chosen]
    engine.client.join_error = errors.InviteRequestSentError(request=None)
    engine.api.select = AsyncMock(return_value=([{"id": "2", "title": chosen.title, "username": "approval"}], "相关", "prompt"))
    engine.enable("search", True)
    await engine.search_once()
    assert engine.store.candidate_status("2") == "pending"
    await engine.search_once()
    assert engine.api.select.await_count == 1


@pytest.mark.asyncio
async def test_stop_then_restart_does_not_join_stale_selection(engine):
    engine.client.chats = [channel(2)]

    async def select(*args):
        engine.enable("search", False)
        engine.enable("search", True)
        return ([{"id": "2", "title": "群", "username": "testgroup"}], "相关", "prompt")

    engine.api.select = select
    engine.enable("search", True)
    await engine.search_once()
    assert not engine.client.joined


@pytest.mark.asyncio
async def test_join_identity_is_checked_again(engine):
    engine.client.chats = [channel(2)]
    engine.client.get_entity = AsyncMock(return_value=channel(999))
    engine.api.select = AsyncMock(return_value=([{"id": "2", "title": "群", "username": "testgroup"}], "相关", "prompt"))
    engine.enable("search", True)
    await engine.search_once()
    assert not engine.client.joined
    assert engine.store.candidate_status("2") == "failed"


@pytest.mark.asyncio
async def test_scheduler_respects_due_time(engine):
    engine.client.dialogs = [channel(1)]
    await engine.refresh_groups()
    engine.enable("send", True)
    engine.next_send = time.time() + 60
    engine.send_once = AsyncMock()
    task = asyncio.create_task(engine.run("send"))
    try:
        await asyncio.sleep(0.05)
        engine.send_once.assert_not_awaited()
        engine.next_send = time.time() - 1
        await asyncio.sleep(1.05)
        engine.send_once.assert_awaited_once()
        assert engine.next_send > time.time() + 290
    finally:
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task


def test_error_messages_do_not_leak_remote_response_or_keys():
    request = httpx.Request("POST", "https://example.test?secret=private")
    response = httpx.Response(401, request=request, text="secret-body")
    message = error_message(httpx.HTTPStatusError("private-key", request=request, response=response))
    assert "401" in message and "private" not in message and "secret" not in message
