#!/bin/bash
# Sub2API 一键部署脚本
# 服务器: 43.134.235.139
# 域名: passiondance.cn
# 部署路径: /opt/sub2api

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}   Sub2API 生产环境部署脚本${NC}"
echo -e "${GREEN}============================================${NC}"

# 检查 root 权限
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}请使用 root 权限运行此脚本${NC}"
    exit 1
fi

# 配置变量
DOMAIN="passiondance.cn"
DEPLOY_DIR="/opt/sub2api"
DATA_DIR="$DEPLOY_DIR/data"
POSTGRES_PASSWORD=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)
TOTP_KEY=$(openssl rand -hex 32)
ADMIN_EMAIL="admin@passiondance.cn"
ADMIN_PASSWORD=$(openssl rand -hex 8)

echo -e "${YELLOW}[1/8] 安装 Docker Compose...${NC}"
# 使用 daocloud 镜像安装
if ! command -v docker-compose &> /dev/null; then
    curl -fsSL https://get.daocloud.io/docker/compose/releases/download/v2.24.5/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    ln -sf /usr/local/bin/docker-compose /usr/bin/docker-compose
    echo -e "${GREEN}Docker Compose 安装完成${NC}"
else
    echo -e "${GREEN}Docker Compose 已存在${NC}"
fi

echo -e "${YELLOW}[2/8] 创建部署目录...${NC}"
mkdir -p $DEPLOY_DIR
mkdir -p $DATA_DIR/postgres_data
mkdir -p $DATA_DIR/redis_data
mkdir -p $DATA_DIR/certs
mkdir -p $DATA_DIR/nginx

echo -e "${YELLOW}[3/8] 生成环境配置文件...${NC}"
cat > $DEPLOY_DIR/.env << 'EOF'
# Server Configuration
BIND_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_MODE=release
RUN_MODE=standard

# Database
POSTGRES_USER=sub2api
POSTGRES_DB=sub2api
DATABASE_SSLMODE=disable

# Admin
ADMIN_EMAIL=admin@passiondance.cn

# 其他配置将在后续步骤填充
EOF

# 更新配置文件
sed -i "s/POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$POSTGRES_PASSWORD/" $DEPLOY_DIR/.env 2>/dev/null || echo "POSTGRES_PASSWORD=$POSTGRES_PASSWORD" >> $DEPLOY_DIR/.env
sed -i "s/JWT_SECRET=.*/JWT_SECRET=$JWT_SECRET/" $DEPLOY_DIR/.env 2>/dev/null || echo "JWT_SECRET=$JWT_SECRET" >> $DEPLOY_DIR/.env
sed -i "s/TOTP_ENCRYPTION_KEY=.*/TOTP_ENCRYPTION_KEY=$TOTP_KEY/" $DEPLOY_DIR/.env 2>/dev/null || echo "TOTP_ENCRYPTION_KEY=$TOTP_KEY" >> $DEPLOY_DIR/.env
sed -i "s/ADMIN_PASSWORD=.*/ADMIN_PASSWORD=$ADMIN_PASSWORD/" $DEPLOY_DIR/.env 2>/dev/null || echo "ADMIN_PASSWORD=$ADMIN_PASSWORD" >> $DEPLOY_DIR/.env

echo -e "${YELLOW}[4/8] 创建 docker-compose.yml...${NC}"
cat > $DEPLOY_DIR/docker-compose.yml << 'EOF'
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    container_name: sub2api-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-sub2api}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}
      POSTGRES_DB: ${POSTGRES_DB:-sub2api}
    volumes:
      - ./data/postgres_data:/var/lib/postgresql/data
    networks:
      - sub2api-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-sub2api}"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: sub2api-redis
    restart: unless-stopped
    volumes:
      - ./data/redis_data:/data
    networks:
      - sub2api-network
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  sub2api:
    image: weishaw/sub2api:latest
    container_name: sub2api
    restart: unless-stopped
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      - AUTO_SETUP=true
      - SERVER_HOST=0.0.0.0
      - SERVER_PORT=8080
      - SERVER_MODE=${SERVER_MODE:-release}
      - RUN_MODE=${RUN_MODE:-standard}
      - DATABASE_HOST=postgres
      - DATABASE_PORT=5432
      - DATABASE_USER=${POSTGRES_USER:-sub2api}
      - DATABASE_PASSWORD=${POSTGRES_PASSWORD}
      - DATABASE_DBNAME=${POSTGRES_DB:-sub2api}
      - DATABASE_SSLMODE=disable
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - ADMIN_EMAIL=${ADMIN_EMAIL:-admin@passiondance.cn}
      - ADMIN_PASSWORD=${ADMIN_PASSWORD}
      - JWT_SECRET=${JWT_SECRET}
      - TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY}
    volumes:
      - ./data:/app/data
    networks:
      - sub2api-network
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  nginx:
    image: nginx:alpine
    container_name: sub2api-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./data/nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./data/certs:/etc/nginx/certs:ro
    networks:
      - sub2api-network
    depends_on:
      - sub2api

networks:
  sub2api-network:
    driver: bridge
EOF

echo -e "${YELLOW}[5/8] 等待证书文件上传...${NC}"
echo -e "${YELLOW}请将证书文件上传到: $DATA_DIR/certs/${NC}"
echo -e "${YELLOW}需要以下文件:${NC}"
echo -e "  - passiondance.cn_bundle.crt (证书)"
echo -e "  - passiondance.cn.key (私钥)"

# 创建临时 nginx 配置（HTTP 模式）
mkdir -p $DATA_DIR/nginx
cat > $DATA_DIR/nginx/nginx.conf << 'EOF'
events {
    worker_connections 1024;
}

http {
    upstream sub2api {
        server sub2api:8080;
    }

    # HTTP 重定向到 HTTPS
    server {
        listen 80;
        server_name passiondance.cn www.passiondance.cn;
        return 301 https://$server_name$request_uri;
    }

    # HTTPS 服务器
    server {
        listen 443 ssl http2;
        server_name passiondance.cn www.passiondance.cn;

        ssl_certificate /etc/nginx/certs/passiondance.cn_bundle.crt;
        ssl_certificate_key /etc/nginx/certs/passiondance.cn.key;
        ssl_session_timeout 1d;
        ssl_session_cache shared:SSL:50m;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;
        ssl_prefer_server_ciphers on;

        # 关键配置：允许带下划线的 header（如 session_id）
        underscores_in_headers on;

        location / {
            proxy_pass http://sub2api;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;

            # WebSocket 支持
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";

            # 超时设置
            proxy_connect_timeout 600s;
            proxy_send_timeout 600s;
            proxy_read_timeout 600s;
        }
    }
}
EOF

echo -e "${YELLOW}[6/8] 保存部署信息...${NC}"
cat > $DEPLOY_DIR/deploy-info.txt << EOF
Sub2API 部署信息
================
部署时间: $(date)
域名: $DOMAIN
部署目录: $DEPLOY_DIR

数据库密码: $POSTGRES_PASSWORD
JWT 密钥: $JWT_SECRET
TOTP 密钥: $TOTP_KEY
管理员邮箱: $ADMIN_EMAIL
管理员密码: $ADMIN_PASSWORD

重要: 请保存好这些信息！

访问地址:
- 前台: https://$DOMAIN/home
- 登录: https://$DOMAIN/login
- 管理后台: https://$DOMAIN/admin/dashboard

命令:
- 启动: cd $DEPLOY_DIR && docker-compose up -d
- 停止: cd $DEPLOY_DIR && docker-compose down
- 查看日志: cd $DEPLOY_DIR && docker-compose logs -f sub2api
- 备份: tar czf sub2api-backup-\$(date +%Y%m%d).tar.gz $DEPLOY_DIR
EOF

echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}部署脚本已生成！${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo -e "${YELLOW}下一步操作:${NC}"
echo -e "1. 上传证书文件到: ${GREEN}$DATA_DIR/certs/${NC}"
echo -e "2. 进入部署目录: ${GREEN}cd $DEPLOY_DIR${NC}"
echo -e "3. 启动服务: ${GREEN}docker-compose up -d${NC}"
echo ""
echo -e "${YELLOW}管理员账号信息已保存到:${NC} ${GREEN}$DEPLOY_DIR/deploy-info.txt${NC}"
