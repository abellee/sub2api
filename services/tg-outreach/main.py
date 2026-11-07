import hashlib
import hmac
import os
import secrets
import time
from contextlib import asynccontextmanager
from pathlib import Path
from urllib.parse import urlsplit

from dotenv import load_dotenv
from fastapi import Depends, FastAPI, HTTPException, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import FileResponse, JSONResponse
from pydantic import BaseModel, Field

from engine import Engine, error_message
from models import Settings, render_prompt
from store import Store

ROOT = Path(__file__).resolve().parent
load_dotenv(ROOT / ".env")


class Login(BaseModel):
    key: str = Field(min_length=1, max_length=2048)


class Phone(BaseModel):
    phone: str = Field(pattern=r"^\+[1-9][0-9]{6,14}$")


class Code(BaseModel):
    code: str = Field(default="", max_length=20)
    password: str = Field(default="", max_length=256)


class Toggle(BaseModel):
    enabled: bool


def create_app(directory=None, admin_key=None, engine_factory=Engine):
    key = admin_key if admin_key is not None else os.getenv("ADMIN_KEY", "")
    directory = Path(directory or os.getenv("DATA_DIR", str(ROOT / "data"))).resolve()
    sessions, attempts = {}, {}

    @asynccontextmanager
    async def lifespan(app):
        if len(key) < 24 or key.startswith("replace-with-"):
            raise RuntimeError("请设置至少 24 位的随机 ADMIN_KEY，不能使用示例值")
        if os.name != "nt":
            os.umask(0o077)
        store = Store(directory)
        app.state.engine = engine_factory(store, directory)
        app.state.store = store
        try:
            await app.state.engine.start()
            yield
        finally:
            await app.state.engine.close()
            store.db.close()

    app = FastAPI(title="Telegram 推广助手", version="1.0.0", lifespan=lifespan,
        docs_url=None, redoc_url=None, openapi_url=None)

    @app.middleware("http")
    async def security(request, call_next):
        if request.method not in ("GET", "HEAD", "OPTIONS"):
            origin = request.headers.get("origin")
            if (origin and urlsplit(origin).netloc != request.headers.get("host")) or request.headers.get("sec-fetch-site") == "cross-site":
                return JSONResponse({"detail": "请从本服务的管理页面操作"}, status_code=403)
        response = await call_next(request)
        response.headers["Cache-Control"] = "no-store"
        response.headers["X-Content-Type-Options"] = "nosniff"
        response.headers["X-Frame-Options"] = "DENY"
        response.headers["Referrer-Policy"] = "no-referrer"
        response.headers["Content-Security-Policy"] = "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"
        return response

    @app.exception_handler(RequestValidationError)
    async def validation_error(request, exc):
        # Pydantic's default response echoes rejected inputs, including passwords.
        fields = [".".join(str(part) for part in error["loc"][1:]) for error in exc.errors()]
        return JSONResponse({"detail": "输入格式不正确，请检查：" + "、".join(fields)}, status_code=422)

    def authenticated(request: Request):
        token = request.cookies.get("tg_admin", "")
        digest = hashlib.sha256(token.encode()).hexdigest()
        if sessions.get(digest, 0) < time.time():
            sessions.pop(digest, None)
            raise HTTPException(401, "请使用管理密钥登录")

    async def perform(label, operation):
        from models import UserError
        try:
            return await operation
        except UserError as exc:
            raise HTTPException(400, str(exc)) from exc
        except Exception as exc:
            app.state.engine.record_error(label, exc)
            raise HTTPException(400, error_message(exc)) from exc

    @app.get("/")
    async def index():
        return FileResponse(ROOT / "static" / "index.html")

    @app.get("/assets/{name}")
    async def asset(name: str):
        if name not in ("app.js", "style.css"):
            raise HTTPException(404)
        return FileResponse(ROOT / "static" / name)

    @app.get("/health")
    async def health():
        return {"ok": True, "version": "1.0.0"}

    @app.post("/api/login")
    async def login(body: Login, request: Request, response: Response):
        stamp = time.time()
        for ident in list(attempts):
            attempts[ident] = [at for at in attempts[ident] if at > stamp - 300]
            if not attempts[ident]:
                attempts.pop(ident)
        peer = request.client.host if request.client else "unknown"
        recent = attempts.get(peer, [])
        if len(recent) >= 5 or len(attempts) >= 4096:
            raise HTTPException(429, "尝试次数太多，请在 5 分钟后重试")
        if not hmac.compare_digest(hashlib.sha256(body.key.encode()).digest(), hashlib.sha256(key.encode()).digest()):
            attempts[peer] = recent + [stamp]
            raise HTTPException(401, "管理密钥错误")
        attempts.pop(peer, None)
        for digest in list(sessions):
            if sessions[digest] < stamp:
                sessions.pop(digest)
        if len(sessions) >= 100:
            sessions.pop(next(iter(sessions)))
        token = secrets.token_urlsafe(32)
        sessions[hashlib.sha256(token.encode()).hexdigest()] = stamp + 43200
        response.set_cookie("tg_admin", token, max_age=43200, httponly=True, samesite="strict",
            secure=os.getenv("COOKIE_SECURE", "false").lower() == "true")
        return {"ok": True}

    @app.post("/api/logout", dependencies=[Depends(authenticated)])
    async def admin_logout(request: Request, response: Response):
        sessions.pop(hashlib.sha256(request.cookies.get("tg_admin", "").encode()).hexdigest(), None)
        response.delete_cookie("tg_admin")
        return {"ok": True}

    @app.get("/api/settings", dependencies=[Depends(authenticated)])
    async def get_settings():
        data = app.state.engine.config.model_dump()
        for field in ("api_key", "telegram_api_hash"):
            data[field + "_saved"] = bool(data[field])
            data[field] = ""
        return data

    @app.put("/api/settings", dependencies=[Depends(authenticated)])
    async def save_settings(body: Settings):
        engine = app.state.engine
        data = body.model_dump()
        async with engine.lock:
            old = engine.config
            for field in ("api_key", "telegram_api_hash"):
                if not data[field]:
                    data[field] = getattr(old, field)
            changed = data["telegram_api_id"] != old.telegram_api_id or data["telegram_api_hash"] != old.telegram_api_hash
            if engine.client and changed:
                if engine.me:
                    raise HTTPException(400, "更换 Telegram API 信息前，请先退出 Telegram 账号")
                await engine.client.disconnect()
                engine.client = None
                engine.phone = engine.phone_hash = None
                engine.awaiting_password = False
            # Only explicit task buttons change running state.
            data.update(send_enabled=old.send_enabled, search_enabled=old.search_enabled)
            if old.send_enabled and not body.message.strip():
                raise HTTPException(400, "发送运行中，消息不能为空")
            if old.send_enabled and not body.selected_group_ids:
                raise HTTPException(400, "发送运行中，不能取消全部目标群；请先停止发送")
            if old.search_enabled and (not body.keywords.strip() or not body.api_base or not body.model or "{{results}}" not in body.prompt):
                raise HTTPException(400, "搜索运行中，关键词、API、模型和 {{results}} 不能为空")
            app.state.store.save_config(data)
        return {"ok": True}

    @app.get("/api/status", dependencies=[Depends(authenticated)])
    async def status():
        return app.state.engine.status()

    @app.post("/api/telegram/code", dependencies=[Depends(authenticated)])
    async def send_code(body: Phone):
        await perform("申请验证码", app.state.engine.send_code(body.phone))
        return {"ok": True}

    @app.post("/api/telegram/login", dependencies=[Depends(authenticated)])
    async def telegram_login(body: Code):
        return await perform("Telegram 登录", app.state.engine.sign_in(body.code, body.password))

    @app.post("/api/telegram/logout", dependencies=[Depends(authenticated)])
    async def telegram_logout():
        await perform("退出 Telegram", app.state.engine.logout())
        return {"ok": True}

    @app.post("/api/groups/refresh", dependencies=[Depends(authenticated)])
    async def refresh():
        await perform("刷新群聊", app.state.engine.refresh_groups())
        return {"ok": True}

    @app.post("/api/models", dependencies=[Depends(authenticated)])
    async def models():
        return {"models": await perform("加载模型列表", app.state.engine.api.models(app.state.engine.config))}

    @app.post("/api/prompt-preview", dependencies=[Depends(authenticated)])
    async def preview():
        config = app.state.engine.config
        return {"prompt": render_prompt(config.prompt, "示例关键词", [{"id": "123456", "title": "示例群", "username": "example", "members": 100}])}

    @app.post("/api/tasks/{kind}", dependencies=[Depends(authenticated)])
    async def toggle(kind: str, body: Toggle):
        from models import UserError
        if kind not in ("send", "search"):
            raise HTTPException(404)
        try:
            app.state.engine.enable(kind, body.enabled)
        except UserError as exc:
            raise HTTPException(400, str(exc)) from exc
        return {"ok": True}

    return app


app = create_app()

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host=os.getenv("HOST", "127.0.0.1"), port=int(os.getenv("PORT", "8788")),
        access_log=False, timeout_graceful_shutdown=20)
