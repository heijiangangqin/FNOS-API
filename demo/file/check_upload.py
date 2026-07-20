import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from common import main_wrap, post

def run(base, token):
    post(base, token, "/api/file/check-upload", {"path": "/vol1/test.txt", "size": 1024, "overwrite": False})

if __name__ == "__main__":
    main_wrap(run)
