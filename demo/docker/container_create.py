import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from common import main_wrap, post

def run(base, token):
    post(base, token, "/api/docker/container/create", {"image": "nginx:latest", "name": "test-nginx", "start": True, "restart": True, "memory": 512, "cpu": 1, "env": ["TZ=Asia/Shanghai"], "port": ["8080:80"], "mount": ["/vol1/html:/usr/share/nginx/html"]})

if __name__ == "__main__":
    main_wrap(run)
