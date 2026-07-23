# ChatRoom Frontend

ChatRoom 的 Vue 3 前端，提供登录、注册和实时聊天工作台。

## 开发

~~~powershell
npm install
npm run dev
~~~

Vite 开发服务器会将以下路径代理至后端 http://localhost:8080：

- /api：REST API
- /ws：WebSocket
- /static：上传文件静态访问

## 环境变量

默认开发模式不需要环境变量。如需部署到不同地址，可在本地 .env.local 中设置：

~~~env
VITE_API_BASE_URL=https://example.com/api/v1
VITE_WS_URL=wss://example.com/ws
~~~

.env.local 已被 Git 忽略，不能写入 token、密码或其他密钥。

## 校验

~~~powershell
npm run build
~~~
