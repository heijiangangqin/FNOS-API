import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from common import main_wrap, get

def run(base, token):
    get(base, token, "/api/ls", {"path": "/"})

if __name__ == "__main__":
    main_wrap(run)
