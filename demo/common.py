import configparser
import json
import sys
import os
import requests

def load_config():
    ini_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "config.ini")
    cfg = configparser.ConfigParser()
    cfg.read(ini_path, encoding="utf-8")
    return cfg["fnos-api"]

def headers(token):
    return {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

def get(base, token, path, params=None):
    r = requests.get(f"{base}{path}", headers=headers(token), params=params, timeout=30)
    print(f"HTTP {r.status_code}")
    print(json.dumps(r.json(), ensure_ascii=False, indent=2))

def post(base, token, path, data=None):
    r = requests.post(f"{base}{path}", headers=headers(token), json=data, timeout=60)
    print(f"HTTP {r.status_code}")
    print(json.dumps(r.json(), ensure_ascii=False, indent=2))

def main_wrap(fn):
    cfg = load_config()
    base = cfg["base_url"].rstrip("/")
    token = cfg["api_token"]
    fn(base, token)
