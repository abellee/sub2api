import asyncio
import time
from contextlib import suppress

import httpx
from telethon import TelegramClient, errors, functions, utils

from models import ModelAPI, Settings, UserError, keywords


def error_message(exc):
    # Never expose Telegram login arguments, API keys, response bodies or request URLs.
    if isinstance(exc, errors.FloodWaitError):
        return f"Telegram 要求等待 {exc.seconds} 秒，所有 Telegram 操作已暂停"
    if isinstance(exc, httpx.HTTPStatusError):
        return f"大模型 API 返回 HTTP {exc.response.status_code}，请检查地址、密钥和模型"
    if isinstance(exc, httpx.RequestError):
        return "大模型 API 连接失败或超时"
    messages = {
        "PhoneCodeInvalidError": "验证码错误", "PhoneCodeExpiredError": "验证码已过期，请重新获取",
        "PasswordHashInvalidError": "两步验证密码错误", "PhoneNumberInvalidError": "手机号格式错误，请包含国家区号",
        "ApiIdInvalidError": "Telegram API ID 或 API Hash 不正确",
        "PhoneNumberBannedError": "该手机号已被 Telegram 限制",
        "ChatWriteForbiddenError": "该群不允许当前账号发送消息",
        "UserBannedInChannelError": "当前账号被该群限制",
        "ChannelsTooMuchError": "已达到 Telegram 入群数量上限",
        "AuthKeyUnregisteredError": "Telegram 登录已失效，请重新登录",
        "SessionRevokedError": "Telegram 登录已被撤销，请重新登录",
    }
    return messages.get(type(exc).__name__, f"操作失败（{type(exc).__name__}）")


class Engine:
    def __init__(self, store, directory, client_factory=TelegramClient, model_api=None):
        self.store, self.directory = store, directory
        self.client_factory = client_factory
        self.api = model_api or ModelAPI()
        self.client = None
        self.me = None
        self.phone = self.phone_hash = None
        self.awaiting_password = False
        self.groups = []
        self.groups_updated = 0
        self.lock = asyncio.Lock()
        self.tasks = []
        self.busy = {"send": False, "search": False}
        self.cooldown = self.store.runtime("cooldown", 0)
        self.next_send = self.store.runtime("next_send", 0)
        self.next_search = self.store.runtime("next_search", 0)
        self.search_index = self.store.runtime("search_index", 0)
        self.last_prompt = ""
        self.search_generation = 0

    @property
    def config(self):
        return Settings(**self.store.config())

    async def start(self):
        if self.config.telegram_api_id and self.config.telegram_api_hash:
            try:
                async with self.lock:
                    await self.connect()
                    if await self.client.is_user_authorized():
                        self.me = await self.client.get_me()
                        if time.time() >= self.cooldown:
                            await self.refresh_groups_locked()
            except Exception as exc:
                self.record_error("恢复 Telegram 登录", exc)
        self.tasks = [asyncio.create_task(self.run(kind)) for kind in ("send", "search")]

    async def close(self):
        for task in self.tasks:
            task.cancel()
        for task in self.tasks:
            with suppress(asyncio.CancelledError):
                await task
        if self.client:
            await self.client.disconnect()
        await self.api.client.aclose()

    async def connect(self):
        config = self.config
        if not config.telegram_api_id or not config.telegram_api_hash:
            raise UserError("请先保存 Telegram API ID 和 API Hash")
        if not self.client:
            self.client = self.client_factory(str(self.directory / "telegram"), config.telegram_api_id,
                config.telegram_api_hash, flood_sleep_threshold=0, request_retries=0)
        if not self.client.is_connected():
            await self.client.connect()

    def check_cooldown(self):
        if time.time() < self.cooldown:
            raise UserError(f"Telegram 冷却中，请等待 {int(self.cooldown - time.time()) + 1} 秒")

    def record_error(self, label, exc):
        if isinstance(exc, errors.FloodWaitError):
            self.cooldown = max(self.cooldown, time.time() + exc.seconds + 1)
            self.store.set_runtime("cooldown", self.cooldown)
        if isinstance(exc, errors.UnauthorizedError):
            self.me = None
        self.store.log(f"{label}：{error_message(exc)}", "error")

    async def send_code(self, phone):
        async with self.lock:
            self.check_cooldown()
            await self.connect()
            if self.me:
                raise UserError("已有 Telegram 账号登录，请先退出账号")
            sent = await self.client.send_code_request(phone)
            self.phone, self.phone_hash = phone, sent.phone_code_hash
            self.awaiting_password = False
        self.store.log("Telegram 验证码已申请，请查看 Telegram 客户端或短信")

    async def sign_in(self, code, password):
        async with self.lock:
            self.check_cooldown()
            await self.connect()
            if not self.phone_hash:
                raise UserError("请先获取验证码；服务重启后需要重新获取")
            try:
                if self.awaiting_password:
                    if not password:
                        raise UserError("请输入 Telegram 两步验证密码")
                    await self.client.sign_in(password=password)
                else:
                    await self.client.sign_in(phone=self.phone, code=code, phone_code_hash=self.phone_hash)
            except errors.SessionPasswordNeededError:
                self.awaiting_password = True
                return {"password_required": True}
            self.me = await self.client.get_me()
            self.phone = self.phone_hash = None
            self.awaiting_password = False
            await self.refresh_groups_locked()
        self.store.log("Telegram 账号登录成功")
        return {"password_required": False}

    async def logout(self):
        async with self.lock:
            if self.client:
                if self.me:
                    self.check_cooldown()
                    if not await self.client.log_out():
                        raise UserError("Telegram 退出未成功，请稍后重试")
                    await self.client.disconnect()
                else:
                    await self.client.disconnect()
                    self.client.session.delete()
            self.client = self.me = None
            self.search_generation += 1
            self.phone = self.phone_hash = None
            self.awaiting_password = False
            self.groups = []
            self.groups_updated = 0
            data = self.config.model_dump()
            data.update(send_enabled=False, search_enabled=False)
            self.store.save_config(data)
        self.store.log("已退出 Telegram 账号，发送和搜索已停止")

    async def refresh_groups_locked(self):
        self.check_cooldown()
        groups = []
        async for dialog in self.client.iter_dialogs():
            if dialog.is_group:
                groups.append({"id": str(utils.get_peer_id(dialog.entity)), "title": dialog.name,
                    "username": getattr(dialog.entity, "username", "") or ""})
        self.groups = sorted(groups, key=lambda item: int(item["id"]))
        self.groups_updated = time.time()

    async def refresh_groups(self):
        async with self.lock:
            if not self.me:
                raise UserError("请先登录 Telegram")
            await self.connect()
            await self.refresh_groups_locked()

    def enable(self, kind, enabled):
        config = self.config
        if enabled:
            if not self.me:
                raise UserError("请先登录 Telegram")
            if kind == "send" and not config.message.strip():
                raise UserError("请先保存要发送的消息")
            if kind == "send" and not any(item["id"] in config.selected_group_ids for item in self.groups):
                raise UserError("请先在群聊列表中勾选发送目标并保存")
            if kind == "search":
                if not keywords(config.keywords) or not config.model or not config.api_base:
                    raise UserError("请先保存关键词、API 地址和模型")
                if "{{results}}" not in config.prompt:
                    raise UserError("筛选提示词必须包含 {{results}} 占位符")
            setattr(self, "next_" + kind, time.time())
            self.store.set_runtime("next_" + kind, getattr(self, "next_" + kind))
        data = config.model_dump()
        data[kind + "_enabled"] = enabled
        if kind == "search":
            self.search_generation += 1
        self.store.save_config(data)
        self.store.log(f"{'定时发送' if kind == 'send' else '搜索并自动入群'}已{'启动' if enabled else '停止'}")

    async def run(self, kind):
        while True:
            try:
                config = self.config
                due = getattr(self, "next_" + kind)
                if getattr(config, kind + "_enabled") and self.me and time.time() >= max(due, self.cooldown):
                    self.busy[kind] = True
                    try:
                        await (self.send_once() if kind == "send" else self.search_once())
                    except Exception as exc:
                        self.record_error("发送" if kind == "send" else "搜索入群", exc)
                    finally:
                        self.busy[kind] = False
                        # Measure from completion: long searches never cause catch-up bursts.
                        next_time = time.time() + getattr(self.config, kind + "_interval")
                        setattr(self, "next_" + kind, next_time)
                        self.store.set_runtime("next_" + kind, next_time)
                await asyncio.sleep(1)
            except asyncio.CancelledError:
                raise
            except Exception as exc:
                self.record_error("任务", exc)
                await asyncio.sleep(10)

    async def send_once(self):
        async with self.lock:
            if not self.config.send_enabled or not self.me:
                return
            self.check_cooldown()
            await self.connect()
            if time.time() - self.groups_updated > 300:
                await self.refresh_groups_locked()
            targets = [item for item in self.groups if item["id"] in self.config.selected_group_ids]
            if not targets:
                self.store.log("勾选的群聊当前不可用，请刷新群聊列表并重新选择", "warning")
                return
            cursor = self.store.runtime("send_cursor", None)
            group = next((item for item in targets if cursor is None or int(item["id"]) > int(cursor)), targets[0])
            try:
                await self.client.send_message(int(group["id"]), self.config.message, parse_mode=None, link_preview=False)
            except errors.FloodWaitError:
                raise
            except Exception:
                self.store.set_runtime("send_cursor", group["id"])
                self.store.log(f"群「{group['title']}」发送失败", "warning")
                raise
            self.store.set_runtime("send_cursor", group["id"])
            self.store.log(f"已发送到「{group['title']}」")

    async def search_once(self):
        config = self.config
        generation = self.search_generation

        def active():
            return self.config.search_enabled and self.me is not None and generation == self.search_generation

        words = keywords(config.keywords)
        if not config.search_enabled or not words or not self.me:
            return
        keyword = words[self.search_index % len(words)]
        self.search_index += 1
        self.store.set_runtime("search_index", self.search_index)
        async with self.lock:
            self.check_cooldown()
            await self.connect()
            await self.refresh_groups_locked()
            result = await self.client(functions.contacts.SearchRequest(q=keyword, limit=config.search_limit))
        joined = {item["id"] for item in self.groups}
        candidates = []
        for chat in result.chats[:config.search_limit]:
            if not getattr(chat, "megagroup", False) or not getattr(chat, "username", None):
                continue
            ident = str(chat.id)
            if str(utils.get_peer_id(chat)) in joined or self.store.candidate_status(ident) == "pending":
                continue
            candidates.append({"id": ident, "title": chat.title, "username": chat.username,
                "members": getattr(chat, "participants_count", None)})
        self.store.log(f"关键词「{keyword}」找到 {len(candidates)} 个可筛选的公开群")
        if not candidates or not active():
            return
        selected, reason, self.last_prompt = await self.api.select(config, keyword, candidates)
        chosen = {item["id"] for item in selected}
        for item in candidates:
            self.store.candidate(item, keyword, "selected" if item["id"] in chosen else "skipped", reason)
        self.store.log(f"模型选中 {len(selected)} 个群")
        for item in selected:
            # Polling the saved flag makes Stop work while waiting between joins.
            while active() and time.time() < max(self.cooldown, self.store.runtime("next_join", 0)):
                await asyncio.sleep(1)
            if not active():
                return
            try:
                async with self.lock:
                    if not active():
                        return
                    self.check_cooldown()
                    await self.connect()
                    entity = await self.client.get_entity(item["username"])
                    if str(entity.id) != item["id"] or not getattr(entity, "megagroup", False):
                        raise UserError("群标识已变化，跳过该群")
                    await self.client(functions.channels.JoinChannelRequest(entity))
                    self.groups_updated = 0
                self.store.candidate(item, keyword, "joined", reason)
                self.store.log(f"已加入「{item['title']}」")
            except errors.InviteRequestSentError:
                self.store.candidate(item, keyword, "pending", "已提交申请，等待群管理员批准")
                self.store.log(f"「{item['title']}」已申请入群，等待批准")
            except errors.FloodWaitError:
                self.store.candidate(item, keyword, "limited", "Telegram 限流，本轮停止入群")
                raise
            except Exception as exc:
                self.store.candidate(item, keyword, "failed", error_message(exc))
                self.record_error(f"加入「{item['title']}」", exc)
                if isinstance(exc, (errors.ChannelsTooMuchError, errors.UnauthorizedError)):
                    self.enable("search", False)
                    return
            finally:
                self.store.set_runtime("next_join", time.time() + self.config.join_interval)
        if active() and time.time() >= self.cooldown:
            await self.refresh_groups()

    def status(self):
        config = self.config
        return {"telegram": {"authorized": self.me is not None,
                "name": ((getattr(self.me, "first_name", "") or "") + " " + (getattr(self.me, "last_name", "") or "")).strip(),
                "username": getattr(self.me, "username", "") or "", "password_required": self.awaiting_password},
            "group_count": len(self.groups), "groups_updated": self.groups_updated,
            "groups": self.groups, "send_enabled": config.send_enabled, "search_enabled": config.search_enabled,
            "busy": self.busy, "next_send": self.next_send, "next_search": self.next_search,
            "cooldown_until": self.cooldown, "last_prompt": self.last_prompt,
            "events": self.store.events(), "candidates": self.store.candidates()}
