import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from common import main_wrap, post

def run(base, token):
    post(base, token, "/api/logger/archive/set", {"switchVal": 1, "filePath": "/vol1/logs", "sizeGt": 100, "dateUnit": 1, "dateBefore": "2024-01-01"})

if __name__ == "__main__":
    main_wrap(run)
