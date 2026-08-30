# Sub2API 本地开发经验记录

## 问题 1：对话返回内容一闪消失（对话消失）

### 现象
前端页面返回数据后立即消失或页面刷新。

### 根本原因
数据库中 `users` 表为空，系统无法验证用户身份，导致前端状态异常。

### 解决方案
1. 创建管理员账号：
   - 直接访问 `/register` 页面注册（首个用户自动成为管理员）
   - 或通过 SQL 直接插入管理员

2. SQL 插入管理员示例：
```sql
-- 生成 bcrypt 密码哈希（Go 代码）
// hash, _ := bcrypt.GenerateFromPassword([]byte("<local-password>"), bcrypt.DefaultCost)

INSERT INTO users (email, password_hash, role, balance, concurrency, status, username)
VALUES (
    'admin@sub2api.org',
    '<replace-with-generated-bcrypt-hash>',
    'admin',
    0,
    5,
    'active',
    'admin'
);
```

### 关键经验
- Sub2API 需要至少一个管理员账号才能正常工作
- 数据库迁移完成后，必须创建初始管理员
- 可以通过 `/setup` 向导、CLI `-setup` 模式或直接 SQL 插入创建

---

## 问题 2：测试连接失败

### 现象
CLI 设置向导运行 `-setup` 时连接数据库失败。

### 错误信息
```
database connection failed: ping failed: pq: role "postgres" does not exist
```

### 根本原因
CLI 设置向导使用默认配置（用户名 postgres），而非读取 `config.yaml` 中的配置。

### 解决方案
1. **推荐**：直接使用 SQL 插入管理员（见上文）
2. 或：修改 `internal/setup/setup.go` 中的默认配置
3. 或：确保 PostgreSQL 中存在 `postgres` 用户

### 关键经验
- CLI 设置向导和运行时服务使用不同的配置读取逻辑
- 向导模式可能不读取 `config.yaml`，而是使用硬编码默认值
- 本地开发建议直接 SQL 插入管理员，避免配置不一致问题

---

## 问题 3：前端构建与依赖

### 现象
前端构建失败或依赖安装报错。

### 根本原因与解决方案

| 问题 | 原因 | 解决 |
|------|------|------|
| `pnpm install` 失败 | npm 和 pnpm 的 node_modules 冲突 | `rm -rf node_modules && pnpm install` |
| CI 构建失败 | pnpm-lock.yaml 未提交 | 每次修改 package.json 后必须提交 pnpm-lock.yaml |
| 构建警告 | 动态导入冲突 | 可忽略（不影响功能） |

### 关键经验
- **必须使用 pnpm**，不能用 npm
- 前端构建命令：`pnpm run build`
- 输出目录：`backend/internal/web/dist/`（嵌入到 Go 二进制中）

---

## 问题 4：Go 依赖下载超时

### 现象
`go build` 或 `go mod download` 超时失败。

### 错误信息
```
dial tcp 142.251.34.209:443: i/o timeout
```

### 解决方案
配置国内 Go 代理：
```bash
export GOPROXY=https://goproxy.cn,direct
```

### 关键经验
- 国内网络建议始终使用 goproxy.cn
- 可将 `export GOPROXY=https://goproxy.cn,direct` 添加到 `~/.zshrc`

---

## 问题 5：PostgreSQL SSL 模式配置

### 现象
后端启动报错：
```
pq: unsupported sslmode "prefer"
```

### 解决方案
修改 `config.yaml`：
```yaml
database:
  sslmode: "disable"  # 开发环境使用 disable
```

### 关键经验
- PostgreSQL@16 在 macOS 上可能不支持 "prefer" 模式
- 生产环境建议使用 "require" 或 "verify-full"

---

## 本地开发快速启动流程

```bash
# 1. 启动 PostgreSQL 和 Redis
brew services start postgresql@16
brew services start redis

# 2. 创建数据库和用户
psql -U $(whoami) -d postgres -c "CREATE USER sub2api WITH PASSWORD 'sub2api';"
psql -U $(whoami) -d postgres -c "CREATE DATABASE sub2api OWNER sub2api;"

# 3. 配置后端
cd backend
cp ../deploy/config.example.yaml config.yaml
# 修改 config.yaml 中的数据库连接信息

# 4. 构建前端
cd ../frontend
pnpm install
pnpm run build

# 5. 构建后端
cd ../backend
export GOPROXY=https://goproxy.cn,direct
go build -tags embed -o sub2api ./cmd/server

# 6. 创建管理员（首次运行）
# 通过 SQL 插入或访问 /register 注册

# 7. 启动服务
./sub2api
```

访问 http://localhost:8080

---

## 账号信息

| 账号 | 邮箱 | 密码 | 角色 |
|------|------|------|------|
| 管理员 | admin@sub2api.org | `<local-password>` | admin |
