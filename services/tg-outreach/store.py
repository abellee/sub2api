import json
import sqlite3
from datetime import datetime, timezone
from pathlib import Path

DEFAULT_PROMPT = '''请根据关键词 {{keyword}}，从下面的 Telegram 群聊中挑选适合发布相关推广内容的群。
候选群聊：
{{results}}
只选择主题相关、允许推广的群；不能确定就不选。
只返回 JSON，格式为 {"join": ["候选群的 id"], "reason": "选择理由"}。'''


class Store:
    def __init__(self, directory: Path):
        directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.db = sqlite3.connect(directory / "state.sqlite3")
        self.db.row_factory = sqlite3.Row
        self.db.executescript('''
            PRAGMA journal_mode=WAL;
            CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS events (
                id INTEGER PRIMARY KEY, at TEXT NOT NULL, level TEXT NOT NULL, message TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS candidates (
                id TEXT PRIMARY KEY, title TEXT NOT NULL, username TEXT NOT NULL,
                keyword TEXT NOT NULL, status TEXT NOT NULL, reason TEXT NOT NULL, updated TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS runtime (key TEXT PRIMARY KEY, value TEXT NOT NULL);
        ''')
        self.db.commit()

    def config(self):
        row = self.db.execute("SELECT value FROM settings WHERE id=1").fetchone()
        return json.loads(row[0]) if row else {}

    def save_config(self, value):
        with self.db:
            self.db.execute("INSERT OR REPLACE INTO settings VALUES (1, ?)", (json.dumps(value),))

    def runtime(self, key, default=None):
        row = self.db.execute("SELECT value FROM runtime WHERE key=?", (key,)).fetchone()
        return json.loads(row[0]) if row else default

    def set_runtime(self, key, value):
        with self.db:
            self.db.execute("INSERT OR REPLACE INTO runtime VALUES (?, ?)", (key, json.dumps(value)))

    def log(self, message, level="info"):
        with self.db:
            self.db.execute("INSERT INTO events(at,level,message) VALUES (?,?,?)", (now(), level, message))
            self.db.execute("DELETE FROM events WHERE id NOT IN (SELECT id FROM events ORDER BY id DESC LIMIT 500)")

    def events(self):
        return [dict(row) for row in self.db.execute("SELECT * FROM events ORDER BY id DESC LIMIT 100")]

    def candidate(self, item, keyword, status, reason=""):
        with self.db:
            self.db.execute("INSERT OR REPLACE INTO candidates VALUES (?,?,?,?,?,?,?)", (
                item["id"], item["title"], item["username"], keyword, status, reason[:2000], now()))
            self.db.execute("DELETE FROM candidates WHERE id NOT IN (SELECT id FROM candidates ORDER BY updated DESC LIMIT 1000)")

    def candidates(self):
        return [dict(row) for row in self.db.execute("SELECT * FROM candidates ORDER BY updated DESC LIMIT 100")]

    def candidate_status(self, ident):
        row = self.db.execute("SELECT status FROM candidates WHERE id=?", (ident,)).fetchone()
        return row[0] if row else None


def now():
    return datetime.now(timezone.utc).isoformat()
