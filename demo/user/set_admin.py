import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from common import main_wrap, post

def run(base, token):
    post(base, token, "/api/user/set-admin", {"user": "testuser", "on": True})

if __name__ == "__main__":
    main_wrap(run)
